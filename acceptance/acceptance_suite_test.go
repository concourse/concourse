package acceptance_test

import (
	"fmt"
	"os"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"

	"testing"
	"time"

	"github.com/concourse/atc"
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
	lockFactory    lock.LockFactory
	teamFactory    db.TeamFactory
	dbProcess      ifrit.Process
	dbListener     *pq.Listener

	defaultTeam  db.Team
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

	agoutiDriver = agouti.PhantomJS()

	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = SynchronizedAfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())

	dbProcess.Signal(os.Interrupt)
	<-dbProcess.Wait()
}, func() {
	err := os.RemoveAll(certTmpDir)
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	postgresRunner.Truncate()
	dbConn = postgresRunner.OpenConn()

	dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

	lockFactory = lock.NewLockFactory(postgresRunner.OpenSingleton())
	teamFactory = db.NewTeamFactory(dbConn, lockFactory)

	var err error
	var found bool
	defaultTeam, found, err = teamFactory.FindTeam(atc.DefaultTeamName)
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue()) // created by postgresRunner

})

var _ = AfterEach(func() {
	Expect(dbConn.Close()).To(Succeed())
	Expect(dbListener.Close()).To(Succeed())
})

func Debug(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")

	logTypes, err := page.LogTypes()
	Expect(err).NotTo(HaveOccurred())
	for _, lt := range logTypes {
		logs, err := page.ReadAllLogs(lt)
		Expect(err).NotTo(HaveOccurred())
		for _, l := range logs {
			fmt.Println("~~~ LOG FROM ", lt+":", l.Message)
		}
	}
}

func init() {
	// satisfy go-unused
	var _ = Debug
}

func Login(page *agouti.Page, baseUrl string) {
	Expect(page.Navigate(baseUrl + "/teams/main/login")).To(Succeed())
	FillLoginFormAndSubmit(page)
}

func FillLoginFormAndSubmit(page *agouti.Page) {
	FillLoginFormWithCredentials(page, "admin", "password")
	Expect(page.FindByButton("login").Click()).To(Succeed())
}

func FillLoginFormWithCredentials(page *agouti.Page, username string, password string) {
	Eventually(page.FindByName("username")).Should(BeFound())
	Expect(page.FindByName("username").Fill(username)).To(Succeed())
	Expect(page.FindByName("password").Fill(password)).To(Succeed())
}

func LoginWithNoAuth(page *agouti.Page, baseUrl string) {
	Expect(page.Navigate(baseUrl + "/teams/main/login")).To(Succeed())
	Eventually(page.FindByButton("login")).Should(BeFound())
	Expect(page.FindByButton("login").Click()).To(Succeed())
	Eventually(page.Find("body")).Should(BeFound())
}
