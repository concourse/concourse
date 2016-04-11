package web_test

import (
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"
	"github.com/pivotal-golang/lager/lagertest"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("BuildsView", func() {
	var build atc.Build

	Context("with a job in the configuration", func() {
		var originGitServer *gitserver.Server

		BeforeEach(func() {
			workers, err := client.ListWorkers()
			Expect(err).NotTo(HaveOccurred())

			logger := lagertest.NewTestLogger("testflight")
			gLog := logger.Session("garden-connection")

			worker := workers[0]
			var gitServerRootfs string
			for _, r := range worker.ResourceTypes {
				if r.Type == "git" {
					gitServerRootfs = r.Image
				}
			}

			originGitServer = gitserver.Start(gitServerRootfs, gclient.New(gconn.NewWithLogger("tcp", worker.GardenAddr, gLog)))
			originGitServer.CommitResource()

			_, _, _, err = client.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
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

			_, err = client.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			build, err = client.CreateJobBuild(pipelineName, "some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("can view resource information of a job build", func() {
			url := atcRoute(fmt.Sprintf("/pipelines/%s/jobs/some-job", pipelineName))

			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page.Find("#page-header.succeeded")).Should(BeFound())

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
				_, err := client.PausePipeline(pipelineName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("displays a blue header", func() {
				Expect(page.Navigate(atcRoute(build.URL))).To(Succeed())

				Expect(page.Navigate(atcRoute(fmt.Sprintf("/pipelines/%s/jobs/job-name/builds/%d", pipelineName, build.ID)))).To(Succeed())

				// top bar should show the pipeline is paused
				Eventually(page.Find(".js-groups.paused"), 10*time.Second).Should(BeFound())
			})
		})
	})

	Context("when manual triggering of the job is disabled", func() {
		var manualTriggerDisabledBuild atc.Build

		BeforeEach(func() {
			_, _, _, err := client.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "job-manual-trigger-disabled",
					},
				},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = client.UnpausePipeline(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			manualTriggerDisabledBuild, err = client.CreateJobBuild(pipelineName, "job-manual-trigger-disabled")
			Expect(err).NotTo(HaveOccurred())

			_, _, pipelineVersion, _, err := client.PipelineConfig(pipelineName)
			Expect(err).NotTo(HaveOccurred())

			_, _, _, err = client.CreateOrUpdatePipelineConfig(pipelineName, pipelineVersion, atc.Config{
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
				"/pipelines/%s/jobs/job-manual-trigger-disabled/builds/%s",
				pipelineName,
				manualTriggerDisabledBuild.Name,
			))))
			Eventually(page.Find("button.build-action"), 10*time.Second).Should(HaveAttribute("disabled", "true"))
		})

		It("should have a disabled button in the job details view", func() {
			Expect(page.Navigate(atcRoute(manualTriggerDisabledBuild.URL))).To(Succeed())

			// job detail w/build info -> job detail
			Eventually(page, 10*time.Second).Should(HaveURL(atcRoute(fmt.Sprintf(
				"/pipelines/%s/jobs/job-manual-trigger-disabled/builds/%s",
				pipelineName,
				manualTriggerDisabledBuild.Name,
			))))

			Eventually(page.Find("h1 a"), 10*time.Second).Should(BeFound())
			Expect(page.Find("h1 a").Click()).To(Succeed())
			Eventually(page, 10*time.Second).Should(HaveURL(atcRoute(fmt.Sprintf(
				"/pipelines/%s/jobs/job-manual-trigger-disabled",
				pipelineName,
			))))

			Expect(page.Find("button.build-action")).ToNot(BeNil())
			Expect(page.Find("button.build-action")).To(HaveAttribute("disabled", "true"))
		})
	})
})
