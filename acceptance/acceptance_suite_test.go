package acceptance_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/agouti"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
	"time"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var (
	atcBin string

	certTmpDir string

	postgresRunner postgresrunner.Runner
	dbConn         db.Conn
	dbProcess      ifrit.Process

	sqlDB *db.SQLDB

	agoutiDriver               *agouti.WebDriver
	tlsCertificateOrganization string
)

var _ = SynchronizedBeforeSuite(func() []byte {
	atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
	Expect(err).NotTo(HaveOccurred())

	return []byte(atcBin)
}, func(b []byte) {
	atcBin = string(b)

	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)

	var err error
	certTmpDir, err = ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())

	tlsCertificateOrganization = "Acme Co"
	err = createCert(certTmpDir)
	Expect(err).NotTo(HaveOccurred())

	postgresRunner = postgresrunner.Runner{
		Port: 5432 + GinkgoParallelNode(),
	}

	dbProcess = ifrit.Invoke(postgresRunner)

	postgresRunner.CreateTestDB()

	if _, err := exec.LookPath("phantomjs"); err == nil {
		fmt.Fprintln(GinkgoWriter, "WARNING: using phantomjs, which is flaky in CI, but is more convenient during development")
		agoutiDriver = agouti.PhantomJS()
	} else {
		agoutiDriver = agouti.Selenium(agouti.Browser("firefox"))
	}

	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())

	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
}, func() {
	err := os.RemoveAll(certTmpDir)
	Expect(err).NotTo(HaveOccurred())
})

func Screenshot(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")
}

func Authenticate(page *agouti.Page, username, password string) {
	header := fmt.Sprintf("%s:%s", username, password)

	page.SetCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(header)),
	})

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	page.Refresh()
}

const BASIC_AUTH = "basic"
const BASIC_AUTH_NO_PASSWORD = "basic-no-password"
const BASIC_AUTH_NO_USERNAME = "basic-no-username"
const GITHUB_AUTH = "github"
const GITHUB_ENTERPRISE_AUTH = "github-enterprise"
const UAA_AUTH = "cf"
const UAA_AUTH_NO_CLIENT_SECRET = "cf-no-secret"
const UAA_AUTH_NO_TOKEN_URL = "cf-no-token-url"
const UAA_AUTH_NO_SPACE = "cf-no-space"
const NOT_CONFIGURED_AUTH = "not-configured"
const DEVELOPMENT_MODE = "dev"
const NO_AUTH = DEVELOPMENT_MODE

func startATC(atcBin string, atcServerNumber uint16, tlsFlags []string, authTypes ...string) (ifrit.Process, uint16, uint16) {
	atcCommand, atcPort, tlsPort := getATCCommand(atcBin, atcServerNumber, tlsFlags, authTypes...)
	atcRunner := ginkgomon.New(ginkgomon.Config{
		Command:       atcCommand,
		Name:          "atc",
		StartCheck:    "atc.listening",
		AnsiColorCode: "32m",
	})
	return ginkgomon.Invoke(atcRunner), atcPort, tlsPort
}

func getATCCommand(atcBin string, atcServerNumber uint16, tlsFlags []string, authTypes ...string) (*exec.Cmd, uint16, uint16) {
	atcPort := 5697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)
	debugPort := 6697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)

	params := []string{
		"--bind-port", fmt.Sprintf("%d", atcPort),
		"--debug-bind-port", fmt.Sprintf("%d", debugPort),
		"--peer-url", fmt.Sprintf("http://127.0.0.1:%d", atcPort),
		"--postgres-data-source", postgresRunner.DataSourceName(),
		"--external-url", fmt.Sprintf("http://127.0.0.1:%d", atcPort),
	}

	for _, authType := range authTypes {
		switch authType {
		case BASIC_AUTH:
			params = append(params,
				"--basic-auth-username", "admin",
				"--basic-auth-password", "password",
			)
		case BASIC_AUTH_NO_PASSWORD:
			params = append(params,
				"--basic-auth-username", "admin",
			)
		case BASIC_AUTH_NO_USERNAME:
			params = append(params,
				"--basic-auth-password", "password",
			)
		case GITHUB_AUTH:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-client-secret", "password",
				"--github-auth-organization", "myorg",
				"--github-auth-team", "myorg/all",
				"--github-auth-user", "myuser",
			)
		case GITHUB_ENTERPRISE_AUTH:
			params = append(params,
				"--github-auth-client-id", "admin",
				"--github-auth-client-secret", "password",
				"--github-auth-organization", "myorg",
				"--github-auth-team", "myorg/all",
				"--github-auth-user", "myuser",
				"--github-auth-auth-url", "https://github.example.com/login/oauth/authorize",
				"--github-auth-token-url", "https://github.example.com/login/oauth/access_token",
				"--github-auth-api-url", "https://github.example.com/api/v3/",
			)
		case UAA_AUTH:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
				"--uaa-auth-cf-space", "myspace",
				"--uaa-auth-auth-url", "https://uaa.example.com/oauth/authorize",
				"--uaa-auth-token-url", "https://uaa.example.com/oauth/token",
				"--uaa-auth-cf-url", "https://cf.example.com/api",
			)
		case UAA_AUTH_NO_CLIENT_SECRET:
			params = append(params,
				"--uaa-auth-client-id", "admin",
			)
		case UAA_AUTH_NO_SPACE:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
			)
		case UAA_AUTH_NO_TOKEN_URL:
			params = append(params,
				"--uaa-auth-client-id", "admin",
				"--uaa-auth-client-secret", "password",
				"--uaa-auth-cf-space", "myspace",
				"--uaa-auth-auth-url", "https://uaa.example.com/oauth/authorize",
				"--uaa-auth-cf-url", "https://cf.example.com/api",
			)
		case DEVELOPMENT_MODE:
			params = append(params, "--development-mode")
		case NOT_CONFIGURED_AUTH:
		default:
			panic("unknown auth type")
		}
	}

	var tlsPort uint16

	if len(tlsFlags) > 0 {
		tlsPort = 7697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)
		params = append(params, "--external-url", fmt.Sprintf("https://127.0.0.1:%d/", tlsPort))

		for _, tlsFlag := range tlsFlags {
			switch tlsFlag {
			case "--tls-bind-port":
				params = append(params, "--tls-bind-port", fmt.Sprintf("%d", tlsPort))
			case "--tls-cert":
				params = append(params, "--tls-cert", filepath.Join(certTmpDir, "server.pem"))
			case "--tls-key":
				params = append(params, "--tls-key", filepath.Join(certTmpDir, "server.key"))
			}
		}
	}

	atcCommand := exec.Command(atcBin, params...)

	return atcCommand, atcPort, tlsPort
}

func createCert(certTmpDir string) error {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{tlsCertificateOrganization},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour),

		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses: []net.IP{
			net.IP{127, 0, 0, 1},
		},
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &(priv.PublicKey), priv)
	if err != nil {
		return err
	}

	certOut, err := os.Create(filepath.Join(certTmpDir, "server.pem"))
	if err != nil {
		return err
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, err := os.OpenFile(filepath.Join(certTmpDir, "server.key"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	pemBlockForKey := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}
	pem.Encode(keyOut, pemBlockForKey)
	keyOut.Close()

	return nil
}
