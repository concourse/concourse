package web_test

import (
	"fmt"
	"time"

	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// . "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
)

var _ = Describe("Viewing resources", func() {
	Describe("a broken resource", func() {
		var brokenResource atc.Resource

		BeforeEach(func() {
			config := atc.Config{
				Resources: []atc.ResourceConfig{
					{
						Name: "broken-resource",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    "i r not reall?",
						},
						CheckEvery: "",
					},
				},
				Jobs: atc.JobConfigs{
					{
						Name: "broken-resource-user",
						Plan: atc.PlanSequence{
							{Get: "broken-resource"},
						},
					},
				},
			}

			byteConfig, err := yaml.Marshal(config)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(pipelineName, "0", byteConfig)
			Expect(err).NotTo(HaveOccurred())

			_, err = team.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			var found bool
			brokenResource, found, err = team.Resource(pipelineName, "broken-resource")
			Expect(found).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("correctly displays logs", func() {
			url := atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/resources/%s", teamName, pipelineName, brokenResource.Name))

			counter := 0
			for {
				Expect(page.Navigate(url)).To(Succeed())
				if counter == 120 {
					Fail("Unable to locate resource log information.")
				}

				if visible, _ := page.Find(".resource-check-status .header i.errored").Visible(); visible {
					break
				}
				counter++
				time.Sleep(500 * time.Millisecond)
			}

			Eventually(page.Find("pre").Text).Should(ContainSubstring("failed: exit status"))
		})
	})
})
