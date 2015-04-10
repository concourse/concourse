package acceptance_test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"github.com/concourse/atc/auth"
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

func Authenticate(page *agouti.Page, username, password string) {
	header := fmt.Sprintf("%s:%s", username, password)

	page.SetCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(header)),
	})

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	RefreshPage(page)
}

func RefreshPage(page *agouti.Page) {
	url, err := page.URL()
	Ω(err).ShouldNot(HaveOccurred())
	Ω(page.Navigate(url)).Should(Succeed())
}
