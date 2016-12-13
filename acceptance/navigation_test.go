package acceptance_test

import (
	"time"

	"code.cloudfoundry.org/urljoiner"

	"github.com/lib/pq"
	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
)

var _ = Describe("Navigation", func() {
	var (
		atcCommand *ATCCommand
		dbListener *pq.Listener
		teamDB     db.TeamDB
	)

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)

		pgxConn := postgresRunner.OpenPgx()
		fakeConnector := new(lockfakes.FakeConnector)
		retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

		lockFactory := lock.NewLockFactory(retryableConn)
		sqlDB = db.NewSQL(dbConn, bus, lockFactory)

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB(atc.DefaultTeamName)
	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	var page *agouti.Page

	BeforeEach(func() {
		var err error
		page, err = agoutiDriver.NewPage()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(page.Destroy()).To(Succeed())
	})

	homepage := func() string {
		return atcCommand.URL("")
	}

	withPath := func(path string) string {
		return urljoiner.Join(homepage(), path)
	}

	Context("with more than one pipeline", func() {
		BeforeEach(func() {
			_, _, err := teamDB.SaveConfig("pipeline-1", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "job-1",
					},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = teamDB.SaveConfig("pipeline-2", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "job-2",
					},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

		})

		Describe("clicking on the home button", func() {
			BeforeEach(func() {
				Expect(page.Navigate(homepage())).To(Succeed())
				Login(page, homepage())
				Eventually(page.FindByLink("job-1")).Should(BeFound())
			})

			It("navigates to the default pipeline when not viewing a pipeline", func() {
				Expect(page.Navigate(withPath("/login"))).To(Succeed())
				Expect(page.FindByClass("fa-home").Click()).To(Succeed())
				Eventually(page.FindByLink("job-1")).Should(BeFound())
			})

			It("navigates to the current pipeline when viewing a non-default pipeline", func() {
				Expect(page.FindByClass("sidebar-toggle").Click()).To(Succeed())
				Eventually(page.FindByLink("pipeline-2")).Should(BeVisible())
				Expect(page.FindByLink("pipeline-2").Click()).To(Succeed())
				Eventually(page.FindByLink("job-2")).Should(BeVisible())
				Expect(page.FindByLink("job-2").Click()).To(Succeed())
				Eventually(page.FindByClass("build-header")).Should(BeVisible())
				Expect(page.FindByClass("fa-home").Click()).To(Succeed())
				Eventually(page.FindByLink("job-2")).Should(BeVisible())
			})
		})
	})
})
