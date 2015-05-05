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
	"github.com/onsi/gomega/gexec"
	. "github.com/sclevine/agouti/matchers"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
)

var _ = Describe("Job Pausing", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16
	var pipelineDBFactory db.PipelineDBFactory
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		var err error
		atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
		Ω(err).ShouldNot(HaveOccurred())

		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener)
		sqlDB = db.NewSQL(dbLogger, dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbLogger, dbConn, bus, sqlDB)
		atcProcess, atcPort = startATC(atcBin, 1)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Ω(dbConn.Close()).Should(Succeed())
		Ω(dbListener.Close()).Should(Succeed())

		postgresRunner.DropTestDB()
	})

	Describe("pausing a job", func() {
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
			return fmt.Sprintf("http://127.0.0.1:%d/pipelines/%s", atcPort, atc.DefaultPipelineName)
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		Context("with a job in the configuration", func() {
			var build db.Build

			BeforeEach(func() {
				var err error
				location := event.OriginLocation{}.Chain(1)

				// job build data
				Ω(sqlDB.SaveConfig(atc.DefaultPipelineName, atc.Config{
					Jobs: []atc.JobConfig{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1))).Should(Succeed())

				pipelineDB, err = pipelineDBFactory.BuildWithName(atc.DefaultPipelineName)
				Ω(err).ShouldNot(HaveOccurred())

				build, err = pipelineDB.CreateJobBuild("job-name")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = sqlDB.StartBuild(build.ID, "", "")
				Ω(err).ShouldNot(HaveOccurred())

				sqlDB.SaveBuildEvent(build.ID, event.Log{
					Origin: event.Origin{
						Name:     "origin-name",
						Type:     event.OriginTypeTask,
						Source:   event.OriginSourceStdout,
						Location: location,
					},
					Payload: "hello this is a payload",
				})

				Ω(sqlDB.FinishBuild(build.ID, db.StatusSucceeded)).Should(Succeed())
			})

			It("can view the resource", func() {
				// homepage -> job detail w/build info
				Expect(page.Navigate(homepage())).To(Succeed())
				// we will need to authenticate later to prove it is working for our page
				Authenticate(page, "admin", "password")
				Eventually(page.FindByLink("job-name")).Should(BeFound())
				Expect(page.FindByLink("job-name").Click()).To(Succeed())

				// job detail w/build info -> job detail
				Expect(page).Should(HaveURL(withPath(fmt.Sprintf("jobs/job-name/builds/%d", build.ID))))
				Expect(page.Find("h1")).To(HaveText(fmt.Sprintf("job-name #%d", build.ID)))
				Expect(page.Find("h1 a").Click()).To(Succeed())
				Expect(page).Should(HaveURL(withPath("jobs/job-name")))

				// job-detail pausing
				Expect(page.Find(".js-job .js-pauseUnpause").Click()).To(Succeed())
				Eventually(page.Find(".js-job .js-pauseUnpause.enabled")).Should(BeFound())
				Eventually(page.Find(".js-job .js-pauseUnpause.disabled")).ShouldNot(BeFound())

				page.Refresh()

				Eventually(page.Find(".js-job .js-pauseUnpause.enabled")).Should(BeFound())
				Eventually(page.Find(".js-job .js-pauseUnpause.disabled")).ShouldNot(BeFound())

				Expect(page.Navigate(homepage())).To(Succeed())
				Eventually(page.Find(".job.paused")).Should(BeFound())

				// job-detail unpausing
				Expect(page.Navigate(withPath("/jobs/job-name"))).To(Succeed())
				Expect(page.Find(".js-job .js-pauseUnpause").Click()).To(Succeed())
				Eventually(page.Find(".js-job .js-pauseUnpause.disabled")).Should(BeFound())
				Eventually(page.Find(".js-job .js-pauseUnpause.enabled")).ShouldNot(BeFound())
			})
		})
	})
})
