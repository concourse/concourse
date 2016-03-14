package acceptance_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"
	. "github.com/sclevine/agouti/matchers"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = Describe("No Pipelines configured", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16
	var page *agouti.Page

	BeforeEach(func() {
		var err error
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)
		atcProcess, atcPort = startATC(atcBin, 1, true, BASIC_AUTH)

		page, err = agoutiDriver.NewPage()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	It("can view the all builds page with no pipelines configured", func() {
		Expect(page.Navigate(fmt.Sprintf("http://127.0.0.1:%d/", atcPort))).To(Succeed())
		Eventually(page.Find(".nav-right .nav-item a")).Should(BeFound())
		Expect(page.Find(".nav-right .nav-item a").Click()).To(Succeed())
		Eventually(page).Should(HaveURL(fmt.Sprintf("http://127.0.0.1:%d/builds", atcPort)))
	})
})
