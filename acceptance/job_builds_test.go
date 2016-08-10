package acceptance_test

import (
	"time"

	"github.com/lib/pq"
	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"code.cloudfoundry.org/gunk/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Job Builds", func() {
	var atcCommand *ATCCommand
	var dbListener *pq.Listener
	var pipelineDBFactory db.PipelineDBFactory
	var pipelineDB db.PipelineDB
	var teamDBFactory db.TeamDBFactory
	var teamName = atc.DefaultTeamName

	BeforeEach(func() {
		postgresRunner.Truncate()
		dbConn = db.Wrap(postgresRunner.Open())
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(dbConn, bus)

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		teamDBFactory = db.NewTeamDBFactory(dbConn, bus)
	})

	AfterEach(func() {
		atcCommand.Stop()

		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())
	})

	Describe("viewing a jobs builds", func() {
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

		Context("with a job in the configuration", func() {
			var pipelineName = "some-pipeline"
			var teamID int

			BeforeEach(func() {
				teamDB := teamDBFactory.GetTeamDB(teamName)
				team, found, err := teamDB.GetTeam()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				teamID = team.ID

				// job build data
				_, _, err = teamDB.SaveConfig(pipelineName, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())

				savedPipeline, found, err := teamDB.GetPipelineByName(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				pipelineDB = pipelineDBFactory.Build(savedPipeline)
			})

			Context("with more then 100 job builds", func() {
				BeforeEach(func() {
					for i := 1; i < 104; i++ {
						_, err := pipelineDB.CreateJobBuild("job-name")
						Expect(err).NotTo(HaveOccurred())
					}
				})

				It("can have paginated results", func() {
					// homepage -> job detail w/build info
					Expect(page.Navigate(homepage())).To(Succeed())
					// we will need to authenticate later to prove it is working for our page
					Login(page, homepage())
					Eventually(page.FindByLink("job-name")).Should(BeFound())
					Expect(page.FindByLink("job-name").Click()).To(Succeed())

					Eventually(page.All("#builds li").Count).Should(Equal(103))

					// job detail w/build info -> job detail
					Eventually(page.Find("h1 a")).Should(BeFound())
					Expect(page.Find("h1 a").Click()).To(Succeed())
					Eventually(page).Should(HaveURL(withPath("/teams/" + teamName + "/pipelines/" + pipelineName + "/jobs/job-name")))
					Eventually(page.All(".js-build").Count).Should(Equal(100))

					Expect(page.First(".pagination .disabled .fa-arrow-left")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-right").Click()).To(Succeed())
					Eventually(page.All(".js-build").Count).Should(Equal(3))

					Expect(page.First(".pagination .disabled .fa-arrow-right")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-left").Click()).To(Succeed())
					Eventually(page.All(".js-build").Count).Should(Equal(100))
				})
			})
		})
	})
})
