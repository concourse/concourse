package acceptance_test

import (
	"database/sql"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

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

var postgresRunner postgresrunner.Runner
var dbConn *sql.DB
var dbProcess ifrit.Process

var sqlDB *db.SQLDB

var agoutiDriver *agouti.WebDriver

var _ = BeforeSuite(func() {
	postgresRunner = postgresrunner.Runner{
		Port: 5432 + GinkgoParallelNode(),
	}

	dbProcess = ifrit.Envoke(postgresRunner)

	agoutiDriver = agouti.PhantomJS()
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())

	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})

func Screenshot(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")
}
