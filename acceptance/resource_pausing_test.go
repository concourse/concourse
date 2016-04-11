package acceptance_test

import (
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/sclevine/agouti"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/cloudfoundry/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Resource Pausing", func() {
	var atcProcess ifrit.Process
	var dbListener *pq.Listener
	var pipelineDB db.PipelineDB
	var atcPort uint16

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)

		sqlDB = db.NewSQL(dbConn, bus)

		atcProcess, atcPort = startATC(atcBin, 1, true, BASIC_AUTH)
		err := sqlDB.DeleteTeamByName("main")
		Expect(err).NotTo(HaveOccurred())
		team, err := sqlDB.SaveTeam(db.Team{
			Name: atc.DefaultTeamName,
			BasicAuth: db.BasicAuth{
				BasicAuthUsername: "admin",
				BasicAuthPassword: "password",
			},
		})
		Expect(err).NotTo(HaveOccurred())

		// job build data
		_, _, err = sqlDB.SaveConfig(team.Name, "some-pipeline", atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "job-name",
					Plan: atc.PlanSequence{
						{
							Get: "resource-name",
						},
					},
				},
			},
			Resources: atc.ResourceConfigs{
				{Name: "resource-name"},
			},
		}, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, sqlDB)

		var found bool
		pipelineDB, found, err = pipelineDBFactory.BuildDefault()
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("pausing a resource", func() {
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

		It("can view the resource", func() {
			// homepage -> resource detail
			Expect(page.Navigate(homepage())).To(Succeed())
			Eventually(page.FindByLink("resource-name")).Should(BeFound())
			Expect(page.FindByLink("resource-name").Click()).To(Succeed())

			// resource detail -> paused resource detail
			Eventually(page).Should(HaveURL(withPath("/pipelines/some-pipeline/resources/resource-name")))
			Expect(page.Find("h1")).To(HaveText("resource-name"))

			Authenticate(page, "admin", "password")

			Expect(page.Find(".js-resource .js-pauseUnpause").Click()).To(Succeed())
			Eventually(page.Find(".header i.fa-play")).Should(BeFound())

			page.Refresh()

			Eventually(page.Find(".header i.fa-play")).Should(BeFound())

			resource, err := pipelineDB.GetResource("resource-name")
			Expect(err).NotTo(HaveOccurred())

			err = pipelineDB.SetResourceCheckError(resource, errors.New("failed to foo the bar"))
			Expect(err).NotTo(HaveOccurred())

			page.Refresh()

			Eventually(page.Find(".header h3")).Should(HaveText("checking failed"))
			Eventually(page.Find(".build-step .step-body")).Should(HaveText("failed to foo the bar"))
		})
	})
})
