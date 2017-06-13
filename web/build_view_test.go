package web_test

import (
	"fmt"

	yaml "gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"

	"github.com/concourse/atc"
)

var _ = Describe("Viewing builds", func() {
	Context("with a job build", func() {
		var build atc.Build

		BeforeEach(func() {
			config := atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Task: "some-task",
								TaskConfig: &atc.LoadTaskConfig{
									TaskConfig: &atc.TaskConfig{
										Platform: "linux",
										ImageResource: &atc.ImageResource{
											Type:   "docker-image",
											Source: atc.Source{"repository": "busybox"},
										},
										Run: atc.TaskRunConfig{
											Path: "sh",
											Args: []string{"-c", "echo hello from some-job"},
										},
									},
								},
							},
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

			build, err = team.CreateJobBuild(pipelineName, "some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("can view the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("teams/%s/pipelines/%s/jobs/some-job/builds/%s", teamName, pipelineName, build.Name))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("some-job #%s", build.Name)))
			Eventually(page.Find("#builds")).Should(HaveText(build.Name))

			Eventually(page.Find(".build-header.succeeded")).Should(BeFound())
			Eventually(page.Find(".build-duration").Text).Should(ContainSubstring("duration"))

			Eventually(page.Find(".build-step .header .succeeded")).Should(BeFound())
			Expect(page.Find(".build-step .header .succeeded").Click()).To(Succeed())
			Eventually(page.Find(".steps").Text).Should(ContainSubstring("hello from some-job"))
		})
	})

	Context("with a one-off build", func() {
		var build atc.Build

		BeforeEach(func() {
			var err error

			pf := atc.NewPlanFactory(0)

			build, err = client.CreateBuild(pf.NewPlan(atc.TaskPlan{
				Name: "some-task",
				Config: &atc.LoadTaskConfig{
					TaskConfig: &atc.TaskConfig{
						Platform: "linux",
						ImageResource: &atc.ImageResource{
							Type:   "docker-image",
							Source: atc.Source{"repository": "busybox"},
						},
						Run: atc.TaskRunConfig{
							Path: "sh",
							Args: []string{"-c", "echo hello from one-off"},
						},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("can view the build", func() {
			Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())
			Eventually(page).Should(HaveURL(atcRoute(fmt.Sprintf("/builds/%d", build.ID))))
			Eventually(page.Find("h1")).Should(HaveText(fmt.Sprintf("build #%d", build.ID)))
			Expect(page.Find("#builds").Text()).To(BeEmpty())

			Eventually(page.Find(".build-header.succeeded")).Should(BeFound())
			Eventually(page.Find(".build-duration").Text).Should(ContainSubstring("duration"))

			Eventually(page.Find(".build-step .header").Text).Should(ContainSubstring("some-task"))
			Eventually(page.Find(".build-step .header .succeeded")).Should(BeFound())
			Expect(page.Find(".build-step .header .succeeded").Click()).To(Succeed())
			Eventually(page.Find(".steps").Text).Should(ContainSubstring("hello from one-off"))
		})
	})
})
