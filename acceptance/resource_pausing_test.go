package acceptance_test

import (
	"errors"

	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"code.cloudfoundry.org/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"
)

var _ = Describe("Resource Pausing", func() {
	var atcCommand *ATCCommand
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		teamFactory := dbng.NewTeamFactory(dbngConn, lockFactory, dbng.NewNoEncryption())
		defaultTeam, found, err := teamFactory.FindTeam(atc.DefaultTeamName)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue()) // created by postgresRunner

		_, _, err = defaultTeam.SavePipeline("some-pipeline", atc.Config{
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
		}, dbng.ConfigVersion(1), dbng.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, BASIC_AUTH)
		err = atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())

		teamDB := teamDBFactory.GetTeamDB(atc.DefaultTeamName)

		savedPipeline, found, err := teamDB.GetPipelineByName("some-pipeline")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)
	})

	AfterEach(func() {
		atcCommand.Stop()
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
			return atcCommand.URL("")
		}

		withPath := func(path string) string {
			return urljoiner.Join(homepage(), path)
		}

		It("can view the resource", func() {
			// homepage -> resource detail
			Login(page, homepage())

			Expect(page.Navigate(homepage())).To(Succeed())

			Eventually(page.Find("#subpage .pipeline-graph")).Should(BeVisible())

			Eventually(page.FindByLink("resource-name")).Should(BeFound())
			Expect(page.FindByLink("resource-name").Click()).To(Succeed())

			// resource detail -> paused resource detail
			Eventually(page).Should(HaveURL(withPath("/teams/main/pipelines/some-pipeline/resources/resource-name")))
			Eventually(page.Find("h1")).Should(HaveText("resource-name"))

			// pause
			Eventually(page.Find(".btn-pause.fl")).Should(BeFound())
			Expect(page.Find(".btn-pause.fl").Click()).To(Succeed())
			Eventually(page.Find(".header i.fa-play")).Should(BeFound())

			Expect(page.Navigate(homepage())).To(Succeed())
			Eventually(page.FindByLink("resource-name")).Should(BeFound())
			Expect(page.FindByLink("resource-name").Click()).To(Succeed())

			// unpause
			Eventually(page.Find(".btn-pause.fl")).Should(BeFound())
			Expect(page.Find(".btn-pause.fl").Click()).To(Succeed())
			Eventually(page.Find(".header i.fa-pause")).Should(BeFound())

			resource, _, err := pipelineDB.GetResource("resource-name")
			Expect(err).NotTo(HaveOccurred())

			err = pipelineDB.SetResourceCheckError(resource, errors.New("failed to foo the bar"))
			Expect(err).NotTo(HaveOccurred())

			page.Refresh()

			Eventually(page.Find(".header h3")).Should(HaveText("checking failed"))
			Eventually(page.Find(".build-step .step-body")).Should(HaveText("failed to foo the bar"))
		})
	})
})
