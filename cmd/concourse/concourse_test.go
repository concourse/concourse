package main_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/concourse/concourse/atc/postgresrunner"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/crypto/ssh"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Web Command", func() {

	var (
		hostKeyFile    string
		hostPubKeyFile string
		configFile     *os.File

		concourseCommand *exec.Cmd
		concourseProcess ifrit.Process
		concourseRunner  *ginkgomon.Runner
		postgresRunner   postgresrunner.Runner
		dbProcess        ifrit.Process
	)

	BeforeEach(func() {
		hostKeyFile, hostPubKeyFile, _, _ = generateSSHKeypair()
		postgresrunner.InitializeRunnerForGinkgo(&postgresRunner, &dbProcess)

		postgresRunner.CreateEmptyTestDB()

		webConfig := fmt.Sprintf(`
web:
  database:
    postgres:
      user: postgres
      database: testdb
      port: %s
  auth:
    main_team:
      local_user:
      - test
    add_local_user:
      test: test
  debug:
    bind_port: %s
  bind_port: %s
  web_server:
    client_id: client-id
    client_secret: client-secret
worker_gateway:
  host_key: %s
  client_id: tsa-client-id
  client_secret: tsa-client-secret
  token_url: "http://localhost/token"
  bind_port: %s`, strconv.Itoa(5433+GinkgoParallelNode()), strconv.Itoa(8000+GinkgoParallelNode()), strconv.Itoa(8080+GinkgoParallelNode()), hostKeyFile, strconv.Itoa(2222+GinkgoParallelNode()))

		var err error
		configFile, err = ioutil.TempFile("", "cmd-test")
		Expect(err).ToNot(HaveOccurred())

		_, err = configFile.Write([]byte(webConfig))
		Expect(err).ToNot(HaveOccurred())

		concourseCommand = exec.Command(
			concoursePath,
			"web",
			"--config="+configFile.Name(),
		)
	})

	JustBeforeEach(func() {
		concourseRunner = ginkgomon.New(ginkgomon.Config{
			Command:       concourseCommand,
			Name:          "web",
			AnsiColorCode: "32m",
		})

		concourseProcess = ifrit.Background(concourseRunner)

		// workaround to avoid panic due to registering http handlers multiple times
		http.DefaultServeMux = new(http.ServeMux)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(concourseProcess)
		<-concourseProcess.Wait()
		postgresRunner.DropTestDB()

		postgresrunner.FinalizeRunnerForGinkgo(&postgresRunner, &dbProcess)
		os.Remove(hostKeyFile)
		os.Remove(hostPubKeyFile)
		os.Remove(filepath.Dir(hostPubKeyFile))
		os.Remove(configFile.Name())
	})

	It("starts atc", func() {
		Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("atc.listening"))
	})

	It("starts tsa", func() {
		Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("tsa.listening"))
	})

	Context("when CONCOURSE_CONCURRENT_REQUEST_LIMIT is invalid", func() {
		BeforeEach(func() {
			concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_CONCURRENT_REQUEST_LIMIT=InvalidAction:0")
		})

		It("prints an error and exits", func() {
			Eventually(concourseRunner.Err()).Should(gbytes.Say("Not a valid route to limit"))
		})
	})

	Context("with CONCOURSE_TSA_CLIENT_ID specified", func() {
		BeforeEach(func() {
			concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_TSA_CLIENT_ID=tsa-client-id")
		})

		It("starts atc", func() {
			Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("atc.listening"))
		})

		It("starts tsa", func() {
			Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("tsa.listening"))
		})

		Context("with CONCOURSE_SYSTEM_CLAIM_KEY is not set to 'aud'", func() {
			BeforeEach(func() {
				concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_SYSTEM_CLAIM_KEY=not-aud")
			})

			It("starts atc", func() {
				Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("atc.listening"))
			})

			It("starts tsa", func() {
				Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("tsa.listening"))
			})
		})

		Context("with CONCOURSE_SYSTEM_CLAIM_KEY set to 'aud'", func() {
			BeforeEach(func() {
				concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_SYSTEM_CLAIM_KEY=aud")
			})

			Context("when the system claim values does not contain the client id", func() {
				BeforeEach(func() {
					concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_SYSTEM_CLAIM_VALUE=system-claim-value-1,system-claim-value-2")
				})

				It("errors", func() {
					Eventually(concourseRunner.Err(), 5*time.Second).Should(
						gbytes.Say("at least one systemClaimValue must be equal to tsa-client-id"),
					)
				})
			})

			Context("when the system claim values contain the client id", func() {
				BeforeEach(func() {
					concourseCommand.Env = append(concourseCommand.Env, "CONCOURSE_SYSTEM_CLAIM_VALUE=system-claim-value-1,tsa-client-id")
				})

				It("starts atc", func() {
					Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("atc.listening"))
				})

				It("starts tsa", func() {
					Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("tsa.listening"))
				})
			})
		})
	})
})

func generateSSHKeypair() (string, string, *rsa.PrivateKey, ssh.PublicKey) {
	path, err := ioutil.TempDir("", "tsa-key")
	Expect(err).NotTo(HaveOccurred())

	privateKeyPath := filepath.Join(path, "id_rsa")
	publicKeyPath := privateKeyPath + ".pub"

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	privateKeyBytes := pem.EncodeToMemory(&pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   x509.MarshalPKCS1PrivateKey(privateKey),
	})

	publicKeyRsa, err := ssh.NewPublicKey(&privateKey.PublicKey)
	Expect(err).NotTo(HaveOccurred())

	publicKeyBytes := ssh.MarshalAuthorizedKey(publicKeyRsa)

	err = ioutil.WriteFile(privateKeyPath, privateKeyBytes, 0600)
	Expect(err).NotTo(HaveOccurred())

	err = ioutil.WriteFile(publicKeyPath, publicKeyBytes, 0600)
	Expect(err).NotTo(HaveOccurred())

	return privateKeyPath, publicKeyPath, privateKey, publicKeyRsa
}
