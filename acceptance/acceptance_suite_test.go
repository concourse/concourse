package acceptance_test

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
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

	agoutiDriver *agouti.WebDriver
)

var _ = SynchronizedBeforeSuite(func() []byte {
	atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
	Expect(err).NotTo(HaveOccurred())

	return []byte(atcBin)
}, func(b []byte) {
	atcBin = string(b)

	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)

	certTmpDir = createCertTmpDir()

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
const DEVELOPMENT_MODE = "dev"
const NO_AUTH = DEVELOPMENT_MODE

func startATC(atcBin string, atcServerNumber uint16, publiclyViewable bool, tlsFlags []string, authTypes ...string) (ifrit.Process, uint16, uint16) {
	atcCommand, atcPort, tlsPort := getATCCommand(atcBin, atcServerNumber, publiclyViewable, tlsFlags, authTypes...)
	atcRunner := ginkgomon.New(ginkgomon.Config{
		Command:       atcCommand,
		Name:          "atc",
		StartCheck:    "atc.listening",
		AnsiColorCode: "32m",
	})
	return ginkgomon.Invoke(atcRunner), atcPort, tlsPort
}

func getATCCommand(atcBin string, atcServerNumber uint16, publiclyViewable bool, tlsFlags []string, authTypes ...string) (*exec.Cmd, uint16, uint16) {
	atcPort := 5697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)
	debugPort := 6697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)

	params := []string{
		"--bind-port", fmt.Sprintf("%d", atcPort),
		"--debug-bind-port", fmt.Sprintf("%d", debugPort),
		"--peer-url", fmt.Sprintf("http://127.0.0.1:%d", atcPort),
		"--postgres-data-source", postgresRunner.DataSourceName(),
	}

	if publiclyViewable {
		params = append(params,
			"--publicly-viewable",
		)
	}

	var tlsPort uint16
	tlsPort = 7697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)
	if len(tlsFlags) > 0 {
		Expect(tlsFlags[0]).To(Equal("--tls-bind-port"))
		params = append(params, "--tls-bind-port", fmt.Sprintf("%d", tlsPort))
	}

	if len(tlsFlags) > 1 {
		Expect(tlsFlags[1]).To(Equal("--tls-cert"))
		params = append(params, "--tls-cert", filepath.Join(certTmpDir, "server.pem"))
	}

	if len(tlsFlags) > 2 {
		Expect(tlsFlags[2]).To(Equal("--tls-key"))
		params = append(params, "--tls-key", filepath.Join(certTmpDir, "server.key"))
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
				"--external-url", "http://example.com",
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
				"--external-url", "http://example.com",
			)
		case DEVELOPMENT_MODE:
			params = append(params, "--development-mode")
		default:
			panic("unknown auth type")
		}
	}

	atcCommand := exec.Command(atcBin, params...)

	return atcCommand, atcPort, tlsPort
}

func createCertTmpDir() string {
	certTmpDir, err := ioutil.TempDir("", "")
	Expect(err).NotTo(HaveOccurred())

	generateCertAndKey := exec.Command(
		"openssl",
		"req",
		"-x509",
		"-subj",
		"/CN=localhost",
		"-nodes",
		"-newkey",
		"rsa:2048",
		"-keyout",
		filepath.Join(certTmpDir, "server.key"),
		"-out",
		filepath.Join(certTmpDir, "server.pem"),
		"-days", "1")
	generateRun, err := gexec.Start(generateCertAndKey, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-generateRun.Exited
	Expect(generateRun.ExitCode()).To(BeZero())

	return certTmpDir
}
