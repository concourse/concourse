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

var _ = Describe("One-off Builds", func() {
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

	Describe("viewing a list of builds", func() {
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
			return fmt.Sprintf("http://127.0.0.1:%d", atcPort)
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		allBuildsListIcon := ".nav-right .nav-item"
		allBuildsListIconLink := ".nav-right .nav-item a"
		firstBuildNumber := ".table-row:nth-of-type(1) .build-number"
		firstBuildLink := ".table-row:nth-of-type(1) a"
		secondBuildLink := ".table-row:nth-of-type(2) a"
		homeLink := ".js-groups li:nth-of-type(2) a"

		Context("with a one off build", func() {
			var oneOffBuild db.Build
			var build db.Build

			BeforeEach(func() {

				// job build data
				_, err := sqlDB.SaveConfig(atc.DefaultPipelineName, atc.Config{
					Jobs: []atc.JobConfig{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				pipelineDB, err = pipelineDBFactory.BuildWithName(atc.DefaultPipelineName)
				Expect(err).NotTo(HaveOccurred())

				build, err = pipelineDB.CreateJobBuild("job-name")
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.StartBuild(build.ID, "exec.v2", `{"Plan":{"id":"some-id","task":{"name":"origin-name"}}}`)
				Expect(err).NotTo(HaveOccurred())

				err = sqlDB.SaveBuildEvent(build.ID, event.Log{
					Origin: event.Origin{
						Name:   "origin-name",
						Type:   event.OriginTypeTask,
						Source: event.OriginSourceStdout,
						ID:     "some-id",
					},
					Payload: "hello this is a payload",
				})
				Expect(err).NotTo(HaveOccurred())

				// One off build data
				oneOffBuild, err = sqlDB.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				_, err = sqlDB.StartBuild(oneOffBuild.ID, "exec.v2", `{"Plan":{"id":"some-other-id","task":{"name":"origin-name"}}}`)
				Expect(err).NotTo(HaveOccurred())

				err = sqlDB.SaveBuildEvent(oneOffBuild.ID, event.Log{
					Origin: event.Origin{
						Name:   "origin-name",
						Type:   event.OriginTypeTask,
						Source: event.OriginSourceStdout,
						ID:     "some-other-id",
					},
					Payload: "hello this is a payload",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("can view builds", func() {
				// homepage -> build list
				Expect(page.Navigate(homepage() + "/pipelines/main")).To(Succeed())
				Eventually(page.Find(allBuildsListIcon)).Should(BeFound())

				Authenticate(page, "admin", "password")

				Expect(page.Find(allBuildsListIconLink).Click()).To(Succeed())

				// build list -> one off build detail
				Eventually(page).Should(HaveURL(withPath("/builds")))
				Expect(page.Find("h1")).To(HaveText("builds"))
				Expect(page.Find(firstBuildNumber).Text()).To(ContainSubstring(fmt.Sprintf("%d", oneOffBuild.ID)))
				Expect(page.Find(firstBuildLink).Click()).To(Succeed())

				// one off build detail
				Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("build #%d", oneOffBuild.ID)))
				Eventually(page.Find("#build-body").Text).Should(ContainSubstring("hello this is a payload"))

				Expect(sqlDB.FinishBuild(oneOffBuild.ID, db.StatusSucceeded)).To(Succeed())
				Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))

				Expect(page.Find(homeLink).Click()).To(Succeed())
				Eventually(page).Should(HaveURL(withPath("/")))

				// one off build detail -> build list
				Expect(page.Find(allBuildsListIconLink).Click()).To(Succeed())

				// job build detail
				Eventually(page.Find(secondBuildLink)).Should(BeFound())
				Expect(page.Find(secondBuildLink).Click()).To(Succeed())
				Eventually(page).Should(HaveURL(withPath(fmt.Sprintf("/pipelines/main/jobs/job-name/builds/%d", build.ID))))
				Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("job-name #%s", build.Name)))
				Expect(page.Find("#builds").Text()).Should(ContainSubstring("%s", build.Name))

				Eventually(page.Find("#build-body").Text).Should(ContainSubstring("hello this is a payload"))

				Expect(sqlDB.FinishBuild(build.ID, db.StatusSucceeded)).To(Succeed())
				Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))
			})

			FIt("can abort builds from the one-off build page", func() {
				// homepage -> build list
				Expect(page.Navigate(homepage() + "/pipelines/main")).To(Succeed())
				Authenticate(page, "admin", "password")
				Expect(page.Find(allBuildsListIconLink).Click()).To(Succeed())

				// build list -> one off build detail
				Eventually(page).Should(HaveURL(withPath("/builds")))
				Expect(page.Find(firstBuildLink).Click()).To(Succeed())

				// one off build detail
				Eventually(page.Find(".build-action-abort")).Should(BeFound())
				Expect(page.Find(".build-action-abort").Click()).To(Succeed())
				Expect(page).Should(HaveURL(withPath(fmt.Sprintf("/builds/%d", oneOffBuild.ID))))

				Eventually(page.Find("#page-header.aborted")).Should(BeFound())
				Eventually(page.Find(".build-action-abort")).ShouldNot(BeFound())
			})
		})
	})
})
