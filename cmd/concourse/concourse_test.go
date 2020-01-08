package main_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/concourse/concourse/atc/postgresrunner"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
	"golang.org/x/crypto/ssh"
)

var _ = Describe("Web Command", func() {

	var (
		args           []string
		hostKeyFile    string
		hostPubKeyFile string

		concourseProcess ifrit.Process
		concourseRunner  *ginkgomon.Runner
	)

	BeforeEach(func() {
		args = []string{"web"}
	})

	JustBeforeEach(func() {
		concourseCommand := exec.Command(
			concoursePath,
			args...,
		)

		concourseRunner = ginkgomon.New(ginkgomon.Config{
			Command:       concourseCommand,
			Name:          "tsa",
			StartCheck:    "atc.cmd.start",
			AnsiColorCode: "32m",
		})

		concourseProcess = ginkgomon.Invoke(concourseRunner)

		// workaround to avoid panic due to registering http handlers multiple times
		http.DefaultServeMux = new(http.ServeMux)
	})

	JustAfterEach(func() {
		ginkgomon.Interrupt(concourseProcess)
		<-concourseProcess.Wait()
	})

	Context("when --tsa-host-key is specified", func() {
		BeforeEach(func() {
			hostKeyFile, hostPubKeyFile, _, _ = generateSSHKeypair()
			args = append(args, "--tsa-host-key", hostKeyFile)
		})

		AfterEach(func() {
			os.Remove(hostKeyFile)
			os.Remove(hostPubKeyFile)
			os.Remove(filepath.Dir(hostPubKeyFile))
		})

		Context("and all other required flags are provided", func() {
			var (
				postgresRunner postgresrunner.Runner
				dbProcess      ifrit.Process
			)

			BeforeEach(func() {
				postgresRunner = postgresrunner.Runner{
					Port: 5433 + GinkgoParallelNode(),
				}
				dbProcess = ifrit.Invoke(postgresRunner)
				postgresRunner.CreateTestDB()

				args = append(args,
					"--postgres-user", "postgres",
					"--postgres-database", "testdb",
					"--postgres-port", strconv.Itoa(5433+GinkgoParallelNode()),
					"--main-team-local-user", "test",
					"--add-local-user", "test:test",
					"--debug-bind-port", strconv.Itoa(8000+GinkgoParallelNode()),
					"--bind-port", strconv.Itoa(8080+GinkgoParallelNode()),
					"--tsa-bind-port", strconv.Itoa(2222+GinkgoParallelNode()),
					"--tsa-client-id", "client-id",
					"--tsa-client-secret", "client-secret",
					"--tsa-token-url", "http://localhost/token",
				)
			})

			AfterEach(func() {
				postgresRunner.DropTestDB()

				dbProcess.Signal(os.Interrupt)
				err := <-dbProcess.Wait()
				Expect(err).NotTo(HaveOccurred())
			})

			It("ATC should start up", func() {
				Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("atc.listening"))
			})

			It("TSA should start up", func() {
				Eventually(concourseRunner.Buffer(), "30s", "2s").Should(gbytes.Say("tsa.listening"))
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
