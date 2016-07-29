package web_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
)

var _ = Describe("Aborting a build", func() {
	Context("with a build in the configuration", func() {
		var build atc.Build

		BeforeEach(func() {
			_, _, _, err := team.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Task: "some-task",
								TaskConfig: &atc.TaskConfig{
									Platform: "linux",
									ImageResource: &atc.ImageResource{
										Type:   "docker-image",
										Source: atc.Source{"repository": "busybox"},
									},
									Run: atc.TaskRunConfig{
										Path: "sh",
										Args: []string{"-c", "echo running; sleep 1000"},
									},
								},
							},
						},
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = team.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			build, err = team.CreateJobBuild(pipelineName, "some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("can abort the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/some-job/builds/%s", teamName, pipelineName, build.Name))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("some-job #%s", build.Name)))

			Eventually(page.Find(".build-action-abort")).Should(BeFound())
			// if we abort before the script has started running, it never actually
			// gets aborted
			Eventually(page.Find(".step-body")).Should(HaveText("running"))
			Expect(page.Find(".build-action-abort").Click()).To(Succeed())

			Eventually(page.Find("#page-header.aborted")).Should(BeFound())
			Eventually(page.Find(".build-action-abort")).ShouldNot(BeFound())
		})
	})

	Context("with a one-off build", func() {
		var build atc.Build

		BeforeEach(func() {
			var err error

			pf := atc.NewPlanFactory(0)

			build, err = client.CreateBuild(pf.NewPlan(atc.TaskPlan{
				Name: "some-task",
				Config: &atc.TaskConfig{
					Platform: "linux",
					ImageResource: &atc.ImageResource{
						Type:   "docker-image",
						Source: atc.Source{"repository": "busybox"},
					},
					Run: atc.TaskRunConfig{
						Path: "sh",
						Args: []string{"-c", "echo running; sleep 1000"},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("can abort the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("builds/%d", build.ID))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("build #%d", build.ID)))

			Eventually(page.Find(".build-action-abort")).Should(BeFound())
			// if we abort before the script has started running, it never actually
			// gets aborted
			Eventually(page.Find(".step-body")).Should(HaveText("running"))
			Expect(page.Find(".build-action-abort").Click()).To(Succeed())

			Eventually(page.Find("#page-header.aborted")).Should(BeFound())
			Eventually(page.Find(".build-action-abort")).ShouldNot(BeFound())
		})
	})
})
