package acceptance_test

import (
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"

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

func Login(page *agouti.Page, homePage string) {
	Expect(page.Navigate(homePage + "/teams/main/login")).To(Succeed())
	Eventually(page.FindByName("username")).Should(BeFound())
	Expect(page.FindByName("username").Fill("admin")).To(Succeed())
	Expect(page.FindByName("password").Fill("password")).To(Succeed())
	Expect(page.FindByButton("login").Click()).To(Succeed())
}
