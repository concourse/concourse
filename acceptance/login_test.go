package acceptance_test

import (
	"fmt"
	"net/url"
	"time"

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

var _ = Describe("Logging In", func() {
	var atcCommand *ATCCommand
	var dbListener *pq.Listener
	var pipelineDBFactory db.PipelineDBFactory
	var pipelineDB db.PipelineDB
	var teamDBFactory db.TeamDBFactory
	var teamName = atc.DefaultTeamName
	var pipelineName = atc.DefaultPipelineName
	var teamDB db.TeamDB

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
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus, lockFactory)

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus, lockFactory)

		teamDB = teamDBFactory.GetTeamDB(teamName)
		_, _, err = teamDB.SaveConfig(pipelineName, atc.Config{
			Jobs: atc.JobConfigs{
				{Name: "job-name"},
			},
		}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	homepage := func() string {
		return atcCommand.URL("")
	}

	Describe("logging in via the UI", func() {
		Context("when user is not logged in", func() {
			var page *agouti.Page

			BeforeEach(func() {
				var err error
				page, err = agoutiDriver.NewPage()
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				Expect(page.Destroy()).To(Succeed())
			})

			Describe("after the user logs in", func() {
				It("should display the pipelines the user has access to in the sidebar", func() {
					Login(page, homepage())
					Expect(page.FindByClass("sidebar-toggle").Click()).To(Succeed())
					Eventually(page.FindByLink("main")).Should(BeVisible())
				})

				It("should no longer display the login link", func() {
					Eventually(page.FindByLink("login")).ShouldNot(BeFound())
				})
			})

			Context("navigating to a team specific page", func() {
				BeforeEach(func() {
					Expect(page.Navigate(atcCommand.URL("/teams/main/pipelines/main"))).To(Succeed())
				})

				It("forces a redirect to /teams/main/login with a redirect query param", func() {
					Eventually(page).Should(HaveURL(atcCommand.URL(fmt.Sprintf("/teams/main/login?redirect=%s", url.QueryEscape("/teams/main/pipelines/main")))))
				})
			})

			Context("when a build exists for an authenticated team", func() {
				var buildPath string

				BeforeEach(func() {
					// job build data
					savedPipeline, found, err := teamDB.GetPipelineByName(pipelineName)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					pipelineDB = pipelineDBFactory.Build(savedPipeline)
					build, err := pipelineDB.CreateJobBuild("job-name")
					Expect(err).NotTo(HaveOccurred())
					buildPath = fmt.Sprintf("/builds/%d", build.ID())
				})

				Context("navigating to a team specific page that exists", func() {
					BeforeEach(func() {
						Expect(page.Navigate(atcCommand.URL(buildPath))).To(Succeed())
					})

					It("forces a redirect to /login", func() {
						Eventually(page).Should(HaveURL(atcCommand.URL(fmt.Sprintf("/login?redirect=%s", url.QueryEscape(buildPath)))))
					})

					It("redirects back to the build page when user logs in", func() {
						Eventually(page.FindByLink(teamName)).Should(BeFound())
						Expect(page.FindByLink(teamName).Click()).To(Succeed())
						FillLoginFormAndSubmit(page)
						Eventually(page).Should(HaveURL(atcCommand.URL(buildPath)))
					})
				})
			})
		})
	})
})
