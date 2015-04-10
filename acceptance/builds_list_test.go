package acceptance_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
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

func startATC(atcBin string) (ifrit.Process, uint16) {
	atcPort := 5697 + uint16(GinkgoParallelNode())
	debugPort := 6697 + uint16(GinkgoParallelNode())

	atcCommand := exec.Command(
		atcBin,
		"-webListenPort", fmt.Sprintf("%d", atcPort),
		"-debugListenPort", fmt.Sprintf("%d", debugPort),
		"-httpUsername", "admin",
		"-httpHashedPassword", "$2a$04$DYaOWeQgyxTCv7QxydTP9u1KnwXWSKipC4BeTuBy.9m.IlkAdqNGG", // "password"
		"-publiclyViewable=true",
		"-templates", filepath.Join("..", "web", "templates"),
		"-public", filepath.Join("..", "web", "public"),
		"-sqlDataSource", postgresRunner.DataSourceName(),
	)
	atcRunner := ginkgomon.New(ginkgomon.Config{
		Command:       atcCommand,
		Name:          "atc",
		StartCheck:    "atc.listening",
		AnsiColorCode: "32m",
	})

	return ginkgomon.Invoke(atcRunner), atcPort
}

var _ = Describe("One-off Builds", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var atcPort uint16

	BeforeEach(func() {
		atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
		Ω(err).ShouldNot(HaveOccurred())

		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		sqlDB = db.NewSQL(dbLogger, dbConn, dbListener)

		atcProcess, atcPort = startATC(atcBin)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Ω(dbConn.Close()).Should(Succeed())
		Ω(dbListener.Close()).Should(Succeed())

		postgresRunner.DropTestDB()
	})

	Describe("viewing a list of builds", func() {
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

		Context("with a one off build", func() {
			var oneOffBuild db.Build
			var build db.Build

			BeforeEach(func() {
				var err error

				location := event.OriginLocation{}.Chain(1)

				// job build data
				Ω(sqlDB.SaveConfig(atc.Config{
					Jobs: []atc.JobConfig{
						{Name: "job-name"},
					},
				}, db.ConfigID(1))).Should(Succeed())

				build, err = sqlDB.CreateJobBuild("job-name")
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

				// One off build data
				oneOffBuild, err = sqlDB.CreateOneOffBuild()
				Ω(err).ShouldNot(HaveOccurred())
				_, err = sqlDB.StartBuild(oneOffBuild.ID, "", "")
				Ω(err).ShouldNot(HaveOccurred())

				sqlDB.SaveBuildEvent(oneOffBuild.ID, event.Log{
					Origin: event.Origin{
						Name:     "origin-name",
						Type:     event.OriginTypeTask,
						Source:   event.OriginSourceStdout,
						Location: location,
					},
					Payload: "hello this is a payload",
				})
			})

			It("can view builds", func() {
				// homepage -> build list
				Expect(page.Navigate(homepage())).To(Succeed())
				Eventually(page.Find(allBuildsListIcon)).Should(BeFound())
				Expect(page.Find(allBuildsListIconLink).Click()).To(Succeed())

				// build list -> one off build detail
				Expect(page).Should(HaveURL(withPath("/builds")))
				Expect(page.Find("h1")).To(HaveText("builds"))
				Expect(page.Find(firstBuildNumber).Text()).To(ContainSubstring(fmt.Sprintf("%d", oneOffBuild.ID)))
				Expect(page.Find(firstBuildLink).Click()).To(Succeed())

				// one off build detail
				Expect(page.Find("h1")).To(HaveText(fmt.Sprintf("build #%d", oneOffBuild.ID)))
				Consistently(page.Find("#build-logs").Text).ShouldNot(ContainSubstring("hello this is a payload"))

				Authenticate(page, "admin", "password")

				Eventually(page.Find("#build-logs").Text).Should(ContainSubstring("hello this is a payload"))
				Expect(page.Find(".abort-build")).Should(BeFound())

				Ω(sqlDB.FinishBuild(oneOffBuild.ID, db.StatusSucceeded)).Should(Succeed())
				Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))

				// one off build detail -> build list
				Expect(page.Find(allBuildsListIconLink).Click()).To(Succeed())

				// job build detail
				Expect(page.Find(secondBuildLink).Click()).To(Succeed())
				Expect(page).Should(HaveURL(fmt.Sprintf("http://127.0.0.1:%d/jobs/job-name/builds/%d", atcPort, build.ID)))
				Expect(page.Find("h1")).To(HaveText(fmt.Sprintf("job-name #%s", build.Name)))
				Expect(page.Find("#builds").Text()).Should(ContainSubstring("%s", build.Name))

				Eventually(page.Find("#build-logs").Text).Should(ContainSubstring("hello this is a payload"))

				Ω(sqlDB.FinishBuild(build.ID, db.StatusSucceeded)).Should(Succeed())

				Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))
			})
		})
	})
})
