package acceptance_test

import (
	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"code.cloudfoundry.org/urljoiner"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Job Builds", func() {
	var atcCommand *ATCCommand

	BeforeEach(func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcCommand.Stop()
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
			var pipeline db.Pipeline

			BeforeEach(func() {
				var err error
				pipeline, _, err = defaultTeam.SavePipeline(pipelineName, atc.Config{
					Jobs: atc.JobConfigs{
						{Name: "job-name"},
					},
				}, db.ConfigVersion(1), db.PipelineUnpaused)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("with more then 100 job builds", func() {
				BeforeEach(func() {
					for i := 1; i < 104; i++ {
						job, found, err := pipeline.Job("job-name")
						Expect(err).ToNot(HaveOccurred())
						Expect(found).To(BeTrue())

						_, err = job.CreateBuild()
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
					Eventually(page).Should(HaveURL(withPath("/teams/" + atc.DefaultTeamName + "/pipelines/" + pipelineName + "/jobs/job-name")))
					Eventually(page.All(".js-build").Count).Should(Equal(100))

					Expect(page.First(".pagination .disabled .fa-arrow-left")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-right")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-right").Click()).To(Succeed())
					Eventually(page.All(".js-build").Count).Should(Equal(3))

					Expect(page.First(".pagination .disabled .fa-arrow-right")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-left")).Should(BeFound())
					Expect(page.First(".pagination .fa-arrow-left").Click()).To(Succeed())

					Eventually(page.All(".js-build").Count).Should(Equal(100))
				})
			})
		})
	})
})
