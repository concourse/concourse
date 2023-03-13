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
	"path/filepath"
	"strings"
	"time"

	"code.cloudfoundry.org/credhub-cli/credhub"
	"code.cloudfoundry.org/credhub-cli/credhub/credentials/values"
	"sigs.k8s.io/yaml"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Credhub", func() {
	BeforeEach(func() {
		if !strings.Contains(string(Bosh("releases").Out.Contents()), "credhub") {
			Skip("credhub release not uploaded")
		}
	})

	Describe("A deployment with credhub", func() {
		var (
			credhubClient   *credhub.CredHub
			credhubInstance *BoshInstance
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
					CA          string `json:"ca"`
					Certificate string `json:"certificate"`
					PrivateKey  string `json:"private_key"`
				} `json:"credhub_client_topgun"`
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

		Describe("/api/v1/info/creds", func() {
			type responseSkeleton struct {
				CredHub struct {
					URL     string   `json:"url"`
					CACerts []string `json:"ca_certs"`
					Health  struct {
						Error    string `json:"error"`
						Response struct {
							Status string `json:"status"`
						} `json:"response"`
						Method string `json:"method"`
					} `json:"health"`
					PathPrefix  string `json:"path_prefix"`
					UAAClientID string `json:"uaa_client_id"`
				} `json:"credhub"`
			}

			var (
				atcURL         string
				parsedResponse responseSkeleton
			)

			BeforeEach(func() {
				atcURL = "http://" + JobInstance("web").IP + ":8080"
			})

			JustBeforeEach(func() {
				token, err := FetchToken(atcURL, AtcUsername, AtcPassword)
				Expect(err).ToNot(HaveOccurred())

				body, err := RequestCredsInfo(atcURL, token.AccessToken)
				Expect(err).ToNot(HaveOccurred())

				err = json.Unmarshal(body, &parsedResponse)
				Expect(err).ToNot(HaveOccurred())
			})

			It("contains credhub config", func() {
				Expect(parsedResponse.CredHub.URL).To(Equal("https://" + credhubInstance.IP + ":8844"))
				Expect(parsedResponse.CredHub.Health.Response).ToNot(BeNil())
				Expect(parsedResponse.CredHub.Health.Response.Status).To(Equal("UP"))
				Expect(parsedResponse.CredHub.Health.Error).To(BeEmpty())
			})
		})

		testCredentialManagement(func() {
			_, err := credhubClient.SetValue("/concourse/main/team_secret", values.Value("some_team_secret"))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/pipeline-creds-test/assertion_script", values.Value(assertionScript))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/pipeline-creds-test/canary", values.Value("some_canary"))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/pipeline-creds-test/resource_type_secret", values.Value("some_resource_type_secret"))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/pipeline-creds-test/resource_secret", values.Value("some_resource_secret"))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetUser("/concourse/main/pipeline-creds-test/job_secret", values.User{
				Username: "some_username",
				Password: "some_password",
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/pipeline-creds-test/resource_version", values.Value("some_exposed_version_secret"))
			Expect(err).ToNot(HaveOccurred())
		}, func() {
			_, err := credhubClient.SetValue("/concourse/main/team_secret", values.Value("some_team_secret"))
			Expect(err).ToNot(HaveOccurred())

			_, err = credhubClient.SetValue("/concourse/main/resource_version", values.Value("some_exposed_version_secret"))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

type Cert struct {
	CA          string `json:"ca"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
}

func generateCredhubCerts(filepath string) (err error) {
	var vars struct {
		CredHubCA           Cert `json:"credhub_ca"`
		CredHubClientAtc    Cert `json:"credhub_client_atc"`
		CredHubClientTopgun Cert `json:"credhub_client_topgun"`
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
		IsCA:                  isCA,
	}

	if isCA {
		parent = &template
	}

	cert, err = x509.CreateCertificate(random, &template, parent, &priv.PublicKey, priv)

	return template, cert, err
}
