package acceptance_test

import (
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/sclevine/agouti"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

var _ = Describe("Resource Pausing", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16
	var pipelineDBFactory db.PipelineDBFactory

	BeforeEach(func() {
		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.Truncate()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbLogger, dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbLogger, dbConn, bus, sqlDB)

		atcProcess, atcPort = startATC(atcBin, 1)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("aborting a build", func() {
		var page *agouti.Page
		var pipelineDB db.PipelineDB

		BeforeEach(func() {
			var err error
			page, err = agoutiDriver.NewPage()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			Expect(page.Destroy()).To(Succeed())
		})

		homepage := func() string {
			return fmt.Sprintf("http://127.0.0.1:%d/pipelines/main", atcPort)
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		Context("with a build in the configuration", func() {
			var build db.Build

			BeforeEach(func() {

				// job build data
				_, err := sqlDB.SaveConfig(atc.DefaultPipelineName, atc.Config{
					Jobs: []atc.JobConfig{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				dbPipeline, err := sqlDB.GetPipelineByName(atc.DefaultPipelineName)
				Expect(err).NotTo(HaveOccurred())
				pipelineDB = pipelineDBFactory.Build(dbPipeline)

				build, err = pipelineDB.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.StartBuild(build.ID, "", "")
				Expect(err).NotTo(HaveOccurred())

				sqlDB.SaveBuildEvent(build.ID, event.Log{
					Origin: event.Origin{
						Name:   "origin-name",
						Type:   event.OriginTypeTask,
						Source: event.OriginSourceStdout,
						ID:     "some-id",
					},
					Payload: "hello this is a payload",
				})
			})

			It("can abort the build", func() {
				// homepage -> job detail w/build info
				Expect(page.Navigate(homepage())).To(Succeed())

				title, err := page.Title()
				Expect(err).NotTo(HaveOccurred())
				Expect(title).To(Equal(fmt.Sprintf("%s - Concourse", atc.DefaultPipelineName)))

				Eventually(page.FindByLink("job-name")).Should(BeFound())
				Expect(page.FindByLink("job-name").Click()).To(Succeed())

				// job detail w/build info -> abort build
				Eventually(page).Should(HaveURL(withPath(fmt.Sprintf("jobs/job-name/builds/%d", build.ID))))
				Expect(page.Find("h1")).To(HaveText(fmt.Sprintf("job-name #%d", build.ID)))

				Expect(page.Find(".js-abortBuild").Click()).To(Succeed())
				Eventually(page).Should(HaveURL(fmt.Sprintf("http://127.0.0.1:%d/login", atcPort)))

				Authenticate(page, "admin", "password")

				Expect(page.Navigate(homepage())).To(Succeed())

				Eventually(page.FindByLink("job-name")).Should(BeFound())
				Expect(page.FindByLink("job-name").Click()).To(Succeed())

				Eventually(page.Find(".js-abortBuild")).Should(BeFound())
				Expect(page.Find(".js-abortBuild").Click()).To(Succeed())
				Expect(page).Should(HaveURL(withPath(fmt.Sprintf("jobs/job-name/builds/%d", build.ID))))

				Eventually(page.Find("#page-header.aborted")).Should(BeFound())
				Eventually(page.Find(".js-abortBuild")).ShouldNot(BeFound())
			})
		})
	})
})
