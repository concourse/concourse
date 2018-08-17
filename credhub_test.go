package topgun_test

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Credhub", func() {
	pgDump := func() *gexec.Session {
		dump := exec.Command("pg_dump", "-U", "atc", "-h", dbInstance.IP, "atc")
		dump.Env = append(os.Environ(), "PGPASSWORD=dummy-password")
		dump.Stdin = bytes.NewBufferString("dummy-password\n")
		session, err := gexec.Start(dump, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	getPipeline := func() *gexec.Session {
		session := spawnFly("get-pipeline", "-p", "pipeline-credhub-test")
		<-session.Exited
		Expect(session.ExitCode()).To(Equal(0))
		return session
	}

	BeforeEach(func() {
		if !strings.Contains(string(bosh("releases").Out.Contents()), "credhub") {
			Skip("credhub release not uploaded")
		}
	})

	Describe("A deployment with credhub", func() {
		var (
			credhubClient   *credhub.CredHub
			credhubInstance *boshInstance
		)

		BeforeEach(func() {
			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/add-empty-credhub.yml",
			)

			credhubInstance = Instance("credhub")
			postgresInstance := JobInstance("postgres")

			varsDir, err := ioutil.TempDir("", "vars")
			Expect(err).ToNot(HaveOccurred())

			defer os.RemoveAll(varsDir)

			varsStore := filepath.Join(varsDir, "vars.yml")
			err = generateCredhubCerts(varsStore)
			Expect(err).ToNot(HaveOccurred())

			Deploy(
				"deployments/concourse.yml",
				"-o", "operations/add-credhub.yml",
				"--vars-store", varsStore,
				"-v", "credhub_ip="+credhubInstance.IP,
				"-v", "postgres_ip="+postgresInstance.IP,
			)

			varsBytes, err := ioutil.ReadFile(varsStore)
			Expect(err).ToNot(HaveOccurred())

			var vars struct {
				CredHubClient struct {
					CA          string `yaml:"ca"`
					Certificate string `yaml:"certificate"`
					PrivateKey  string `yaml:"private_key"`
				} `yaml:"credhub_client_topgun"`
			}

			err = yaml.Unmarshal(varsBytes, &vars)
			Expect(err).ToNot(HaveOccurred())

			clientCert := filepath.Join(varsDir, "client.cert")
			err = ioutil.WriteFile(clientCert, []byte(vars.CredHubClient.Certificate), 0644)
			Expect(err).ToNot(HaveOccurred())

			clientKey := filepath.Join(varsDir, "client.key")
			err = ioutil.WriteFile(clientKey, []byte(vars.CredHubClient.PrivateKey), 0644)
			Expect(err).ToNot(HaveOccurred())

			credhubClient, err = credhub.New(
				"https://"+credhubInstance.IP+":8844",
				credhub.CaCerts(vars.CredHubClient.CA),
				credhub.ClientCert(clientCert, clientKey),
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("/api/v1/info/creds", func() {
			type responseSkeleton struct {
				CredHub struct {
					Url     string   `json:"url"`
					CACerts []string `json:"ca_certs"`
					Health  struct {
						Error    string `json:"error"`
						Response struct {
							Status string `json:"status"`
						} `json:"response"`
						Method string `json:"method"`
					} `json:"health"`
					PathPrefix  string `json:"path_prefix"`
					UAAClientId string `json:"uaa_client_id"`
				} `json:"credhub"`
			}

			var (
				atcUrl         string
				parsedResponse responseSkeleton
			)

			BeforeEach(func() {
				atcUrl = "http://" + jobInstances["atc"][0].IP + ":8080"
			})

			JustBeforeEach(func() {
				body, err := requestCredsInfo(atcUrl)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &parsedResponse)
				Expect(err).ToNot(HaveOccurred())
			})

			It("contains credhub config", func() {
				Expect(parsedResponse.CredHub.Url).To(Equal("https://" + credhubInstance.IP + ":8844"))
				Expect(parsedResponse.CredHub.Health.Response).ToNot(BeNil())
				Expect(parsedResponse.CredHub.Health.Response.Status).To(Equal("UP"))
				Expect(parsedResponse.CredHub.Health.Error).To(BeEmpty())
			})

		})

		Context("with a pipeline build", func() {
			BeforeEach(func() {
				_, err := credhubClient.SetValue("/concourse/main/pipeline-credhub-test/resource_type_repository", values.Value("concourse/time-resource"))
				Expect(err).ToNot(HaveOccurred())

				credhubClient.SetValue("/concourse/main/pipeline-credhub-test/time_resource_interval", values.Value("10m"))
				credhubClient.SetUser("/concourse/main/pipeline-credhub-test/job_secret", values.User{
					Username: "Hello",
					Password: "World",
				})
				credhubClient.SetValue("/concourse/main/team_secret", values.Value("Sauce"))
				credhubClient.SetValue("/concourse/main/pipeline-credhub-test/image_resource_repository", values.Value("busybox"))

				By("setting a pipeline that contains credhub secrets")
				fly("set-pipeline", "-n", "-c", "pipelines/credential-management.yml", "-p", "pipeline-credhub-test")

				By("getting the pipeline config")
				session := getPipeline()
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("concourse/time-resource"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("10m"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Hello/World"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("Sauce"))
				Expect(string(session.Out.Contents())).ToNot(ContainSubstring("busybox"))

				By("unpausing the pipeline")
				fly("unpause-pipeline", "-p", "pipeline-credhub-test")
			})

			It("parameterizes via Credhub and leaves the pipeline uninterpolated", func() {
				By("triggering job")
				watch := spawnFly("trigger-job", "-w", "-j", "pipeline-credhub-test/job-with-custom-input")
				wait(watch)
				Expect(watch).To(gbytes.Say("GET SECRET: GET-Hello/GET-World"))
				Expect(watch).To(gbytes.Say("PUT SECRET: PUT-Hello/PUT-World"))
				Expect(watch).To(gbytes.Say("GET SECRET: PUT-GET-Hello/PUT-GET-World"))
				Expect(watch).To(gbytes.Say("SECRET: Hello/World"))
				Expect(watch).To(gbytes.Say("TEAM SECRET: Sauce"))

				By("taking a dump")
				session := pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).ToNot(gbytes.Say("10m"))
				Expect(session).To(gbytes.Say("Hello/World")) // build echoed it; nothing we can do
				Expect(session).To(gbytes.Say("Sauce"))       // build echoed it; nothing we can do
				Expect(session).ToNot(gbytes.Say("busybox"))
			})

			Context("when the job's inputs are used for a one-off build", func() {
				It("parameterizes the values using the job's pipeline scope", func() {
					By("triggering job to populate its inputs")
					watch := spawnFly("trigger-job", "-w", "-j", "pipeline-credhub-test/job-with-input")
					wait(watch)
					Expect(watch).To(gbytes.Say("GET SECRET: GET-Hello/GET-World"))
					Expect(watch).To(gbytes.Say("PUT SECRET: PUT-Hello/PUT-World"))
					Expect(watch).To(gbytes.Say("GET SECRET: PUT-GET-Hello/PUT-GET-World"))
					Expect(watch).To(gbytes.Say("SECRET: Hello/World"))
					Expect(watch).To(gbytes.Say("TEAM SECRET: Sauce"))

					By("executing a task that parameterizes image_resource")
					watch = spawnFly("execute", "-c", "tasks/credential-management-with-job-inputs.yml", "-j", "pipeline-credhub-test/job-with-input")
					wait(watch)
					Expect(watch).To(gbytes.Say("./some-resource/input"))

					By("taking a dump")
					session := pgDump()
					Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
					Expect(session).ToNot(gbytes.Say("10m"))
					Expect(session).To(gbytes.Say("./some-resource/input")) // build echoed it; nothing we can do
				})
			})
		})

		Context("with a one-off build", func() {
			BeforeEach(func() {
				_, err := credhubClient.SetValue("/concourse/main/task_secret", values.Value("Hiii"))
				Expect(err).ToNot(HaveOccurred())

				credhubClient.SetValue("/concourse/main/image_resource_repository", values.Value("busybox"))
			})

			It("parameterizes image_resource and params in a task config", func() {
				By("executing a task that parameterizes image_resource")
				watch := spawnFly("execute", "-c", "tasks/credential-management.yml")
				wait(watch)
				Expect(watch).To(gbytes.Say("SECRET: Hiii"))

				By("taking a dump")
				session := pgDump()
				Expect(session).ToNot(gbytes.Say("concourse/time-resource"))
				Expect(session).To(gbytes.Say("Hiii")) // build echoed it; nothing we can do
			})
		})
	})
})

type Cert struct {
	CA          string `yaml:"ca"`
	Certificate string `yaml:"certificate"`
	PrivateKey  string `yaml:"private_key"`
}

func generateCredhubCerts(filepath string) (err error) {
	var vars struct {
		CredHubCA           Cert `yaml:"credhub_ca"`
		CredHubClientAtc    Cert `yaml:"credhub_client_atc"`
		CredHubClientTopgun Cert `yaml:"credhub_client_topgun"`
	}

	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	// root ca cert
	rootCaTemplate, rootCaCert, err := generateCert("credhubCA", "", true, x509.ExtKeyUsageServerAuth, nil, key)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)

	pem.Encode(writer, &pem.Block{Type: "CERTIFICATE", Bytes: rootCaCert})
	writer.Flush()
	rootCa := b.String()
	b.Reset()

	pem.Encode(writer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	writer.Flush()
	rootCaKey := b.String()
	b.Reset()

	vars.CredHubCA.CA = rootCa
	vars.CredHubCA.Certificate = rootCa
	vars.CredHubCA.PrivateKey = rootCaKey

	// client topgun cert
	_, clientTopgunCert, err := generateCert("credhubCA", "app:eef9440f-7d2b-44b4-99e2-a619cbec99e6", false, x509.ExtKeyUsageClientAuth, &rootCaTemplate, key)
	if err != nil {
		return err
	}

	pem.Encode(writer, &pem.Block{Type: "CERTIFICATE", Bytes: clientTopgunCert})
	writer.Flush()
	clientTopgun := b.String()
	b.Reset()

	vars.CredHubClientTopgun.CA = rootCa
	vars.CredHubClientTopgun.Certificate = clientTopgun
	vars.CredHubClientTopgun.PrivateKey = rootCaKey

	// client atc cert
	_, clientAtcCert, err := generateCert("concourse", "app:df4d7e2c-edfa-432d-ab7e-ee97846b06d0", false, x509.ExtKeyUsageClientAuth, &rootCaTemplate, key)
	if err != nil {
		return err
	}

	pem.Encode(writer, &pem.Block{Type: "CERTIFICATE", Bytes: clientAtcCert})
	writer.Flush()
	clientAtc := b.String()
	b.Reset()

	vars.CredHubClientAtc.CA = rootCa
	vars.CredHubClientAtc.Certificate = clientAtc
	vars.CredHubClientAtc.PrivateKey = rootCaKey

	varsYaml, _ := yaml.Marshal(&vars)
	ioutil.WriteFile(filepath, varsYaml, 0644)
	return nil
}

func generateCert(commonName string, orgUnit string, isCA bool, extKeyUsage x509.ExtKeyUsage, parent *x509.Certificate, priv *rsa.PrivateKey) (template x509.Certificate, cert []byte, err error) {

	random := rand.Reader
	now := time.Now()
	then := now.Add(60 * 60 * 24 * 1000 * 1000 * 1000) // 24 hours

	template = x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         commonName,
			Organization:       []string{"Cloud Foundry"},
			OrganizationalUnit: []string{orgUnit},
		},
		NotBefore: now,
		NotAfter:  then,

		SubjectKeyId: []byte{1, 2, 3, 4},
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{extKeyUsage},

		BasicConstraintsValid: true,
		IsCA: isCA,
	}

	if isCA {
		parent = &template
	}

	cert, err = x509.CreateCertificate(random, &template, parent, &priv.PublicKey, priv)

	return template, cert, err
}
