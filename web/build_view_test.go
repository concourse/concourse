package acceptance_test

import (
	"fmt"

	"github.com/sclevine/agouti"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
)

var _ = Describe("Viewing builds", func() {
	var page *agouti.Page

	BeforeEach(func() {
		var err error
		page, err = agoutiDriver.NewPage()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(page.Destroy()).To(Succeed())
	})

	Context("with a job build", func() {
		var build atc.Build

		BeforeEach(func() {
			_, _, err := client.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Task: "some-task",
								TaskConfig: &atc.TaskConfig{
									Run: atc.TaskRunConfig{
										Path: "sh",
										Args: []string{"-c", "echo hello from some-job"},
									},
								},
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = client.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			build, err = client.CreateJobBuild(pipelineName, "some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("can view the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("pipelines/%s/jobs/some-job/builds/%s", pipelineName, build.Name))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("some-job #%s", build.Name)))
			Expect(page.Find("#builds").Text()).Should(ContainSubstring(build.Name))

			Eventually(page.Find("#page-header.succeeded")).Should(BeFound())
			Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))

			Expect(page.Find(".build-step .header").Click()).To(Succeed())
			Eventually(page.Find("#build-body").Text).Should(ContainSubstring("hello from some-job"))
		})
	})

	Context("with a one-off build", func() {
		var build atc.Build

		BeforeEach(func() {
			var err error

			build, err = client.CreateBuild(atc.Plan{
				Task: &atc.TaskPlan{
					Name: "some-task",
					Config: &atc.TaskConfig{
						Run: atc.TaskRunConfig{
							Path: "sh",
							Args: []string{"-c", "echo hello from one-off"},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("can view the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("/builds/%d", build.ID))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("build #%d", build.ID)))
			Expect(page.Find("#builds").Text()).Should(BeEmpty())

			Eventually(page.Find("#page-header.succeeded")).Should(BeFound())
			Eventually(page.Find(".build-times").Text).Should(ContainSubstring("duration"))

			Expect(page.Find(".build-step .header").Click()).To(Succeed())
			Eventually(page.Find("#build-body").Text).Should(ContainSubstring("hello from one-off"))
		})
	})
})
