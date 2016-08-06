package web_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("BuildsView", func() {
	var build atc.Build

	Context("with a job in the configuration", func() {
		var originGitServer *gitserver.Server

		BeforeEach(func() {
			originGitServer = gitserver.Start(client)
			originGitServer.CommitResource()

			_, _, _, err := team.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get: "some-input-resource",
							},
							{
								Task: "some-task",
								TaskConfig: &atc.TaskConfig{
									Platform: "linux",
									ImageResource: &atc.ImageResource{
										Type:   "docker-image",
										Source: atc.Source{"repository": "busybox"},
									},
									Inputs: []atc.TaskInputConfig{
										{Name: "some-input-resource"},
									},
									Outputs: []atc.TaskOutputConfig{
										{Name: "some-output-src"},
									},
									Run: atc.TaskRunConfig{
										Path: "cp",
										Args: []string{"-r", "some-input-resource/.", "some-output-src"},
									},
								},
							},
							{
								Put:    "some-output-resource",
								Params: atc.Params{"repository": "some-output-src"},
							},
						},
					},
				},
				Resources: []atc.ResourceConfig{
					{
						Name: "some-input-resource",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    originGitServer.URI(),
						},
						CheckEvery: "",
					},
					{
						Name: "some-output-resource",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    originGitServer.URI(),
						},
						CheckEvery: "",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = team.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			build, err = team.CreateJobBuild(pipelineName, "some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			originGitServer.Stop()
		})

		It("can view resource information of a job build", func() {
			url := atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/jobs/some-job", teamName, pipelineName))

			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page.Find(".build-header.succeeded")).Should(BeFound())

			Eventually(page.All(".builds-list li")).Should(HaveCount(1))

			Expect(page.Find(".builds-list li:first-child a")).To(HaveText("#1"))
			Eventually(page.Find(".builds-list li:first-child a.succeeded"), 10*time.Second).Should(BeFound())

			buildTimes, err := page.Find(".builds-list li:first-child .build-duration").Text()
			Expect(err).NotTo(HaveOccurred())
			Expect(buildTimes).To(ContainSubstring("started"))
			Expect(buildTimes).To(MatchRegexp("started \\d+s ago"))
			Expect(buildTimes).To(MatchRegexp("finished \\d+s ago"))
			Expect(buildTimes).To(MatchRegexp("duration \\d+s"))

			Eventually(page.Find(".builds-list li:first-child .inputs .resource-name"), 10*time.Second).Should(BeFound())
			Expect(page.Find(".builds-list li:first-child .inputs .resource-name")).To(HaveText("some-input-resource"))
			Expect(page.Find(".builds-list li:first-child .inputs .resource-version .dict-key")).To(HaveText("ref"))
			Expect(page.Find(".builds-list li:first-child .inputs .resource-version .dict-value")).To(MatchText("[0-9a-f]{40}"))

			Expect(page.Find(".builds-list li:first-child .outputs .resource-name")).To(HaveText("some-output-resource"))
			Expect(page.Find(".builds-list li:first-child .outputs .resource-version .dict-key")).To(HaveText("ref"))
			Expect(page.Find(".builds-list li:first-child .outputs .resource-version .dict-value")).To(MatchText("[0-9a-z]{40}"))

			// button should not have the boolean attribute "disabled" set. agouti currently returns
			// an empty string in that case.
			Expect(page.Find("button.build-action")).ToNot(BeNil())
			Expect(page.Find("button.build-action")).To(HaveAttribute("disabled", ""))
		})

		Describe("paused pipeline", func() {
			BeforeEach(func() {
				_, err := team.PausePipeline(pipelineName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("displays a blue header", func() {
				Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())

				Expect(page.Navigate(atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/jobs/some-job/builds/%s", teamName, pipelineName, build.Name)))).To(Succeed())

				// top bar should show the pipeline is paused
				Eventually(page.Find(".js-top-bar.paused"), 10*time.Second).Should(BeFound())
			})
		})
	})

	Context("when manual triggering of the job is disabled", func() {
		var manualTriggerDisabledBuild atc.Build

		BeforeEach(func() {
			_, _, _, err := team.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "job-manual-trigger-disabled",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = team.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			manualTriggerDisabledBuild, err = team.CreateJobBuild(pipelineName, "job-manual-trigger-disabled")
			Expect(err).NotTo(HaveOccurred())

			_, _, pipelineVersion, _, err := team.PipelineConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = team.CreateOrUpdatePipelineConfig(pipelineName, pipelineVersion, atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name:                 "job-manual-trigger-disabled",
						DisableManualTrigger: true,
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should have a disabled button in the build details view", func() {
			Expect(page.Navigate(atcRoute(manualTriggerDisabledBuild.URL))).To(Succeed())

			// job detail w/build info -> job detail
			Eventually(page, 10*time.Second).Should(HaveURL(atcRoute(fmt.Sprintf(
				"/teams/%s/pipelines/%s/jobs/job-manual-trigger-disabled/builds/%s",
				teamName,
				pipelineName,
				manualTriggerDisabledBuild.Name,
			))))
			Eventually(page.Find("button.build-action"), 10*time.Second).Should(HaveAttribute("disabled", "true"))
		})

		It("should have a disabled button in the job details view", func() {
			Expect(page.Navigate(atcRoute(manualTriggerDisabledBuild.URL))).To(Succeed())

			// job detail w/build info -> job detail
			Eventually(page, 10*time.Second).Should(HaveURL(atcRoute(fmt.Sprintf(
				"/teams/%s/pipelines/%s/jobs/job-manual-trigger-disabled/builds/%s",
				teamName,
				pipelineName,
				manualTriggerDisabledBuild.Name,
			))))

			Eventually(page.Find("h1 a"), 10*time.Second).Should(BeFound())
			Expect(page.Find("h1 a").Click()).To(Succeed())
			Eventually(page, 10*time.Second).Should(HaveURL(atcRoute(fmt.Sprintf(
				"/teams/%s/pipelines/%s/jobs/job-manual-trigger-disabled",
				teamName,
				pipelineName,
			))))

			Eventually(page.Find("button.build-action")).Should(BeFound())
			Expect(page.Find("button.build-action")).To(HaveAttribute("disabled", "true"))
		})
	})
})
