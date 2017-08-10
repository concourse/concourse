package acceptance_test

import (
	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Navigation", func() {
	var atcCommand *ATCCommand

	BeforeEach(func() {
		atcCommand = NewATCCommand(atcBin, 1, postgresRunner.DataSourceName(), []string{}, false, BASIC_AUTH)
		err := atcCommand.Start()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		atcCommand.Stop()
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

	// withPath := func(path string) string {
	// 	return urljoiner.Join(homepage(), path)
	// }

	Context("with more than one pipeline", func() {
		BeforeEach(func() {
			_, _, err := defaultTeam.SavePipeline("pipeline-1", atc.Config{
				Jobs: atc.JobConfigs{
					{
						Name: "job-1",
					},
				},
			}, db.ConfigVersion(1), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = defaultTeam.SavePipeline("pipeline-2", atc.Config{
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

			// pending #133520341
			// FIt("navigates to the default pipeline when not viewing a pipeline", func() {
			// 	Expect(page.Navigate(withPath("/login"))).To(Succeed())
			// 	Expect(page.FindByClass("fa-home").Click()).To(Succeed())
			// 	Eventually(page.FindByLink("job-1")).Should(BeFound())
			// })

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
