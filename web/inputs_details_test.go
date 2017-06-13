package web_test

import (
	"fmt"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/testflight/gitserver"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sclevine/agouti/matchers"
)

var _ = Describe("InputDetails", func() {
	var originGitServer *gitserver.Server
	var build atc.Build

	BeforeEach(func() {
		originGitServer = gitserver.Start(client)
		originGitServer.CommitResource()
	})

	AfterEach(func() {
		originGitServer.Stop()
	})

	Context("when pinned version is unavailable", func() {
		BeforeEach(func() {
			config := atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get:     "some-pinned-resource",
								Version: &atc.VersionConfig{Pinned: atc.Version{"ref": "unknown-version"}},
							},
						},
					},
				},
				Resources: []atc.ResourceConfig{
					{
						Name: "some-pinned-resource",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    originGitServer.URI(),
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

		It("displays input details", func() {
			url := atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/jobs/some-job/builds/%s", teamName, pipelineName, build.Name))

			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page.All(".details li"), 60*time.Second).Should(HaveCount(1))
			Expect(page.Find(".details li:first-child")).To(HaveText(`some-pinned-resource - pinned version {"ref":"unknown-version"} is not available`))
		})
	})

	Context("when no versions are available", func() {
		BeforeEach(func() {
			config := atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get: "some-resource-with-no-versions",
							},
						},
					},
				},
				Resources: []atc.ResourceConfig{
					{
						Name: "some-resource-with-no-versions",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    "broken-resource",
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

		It("displays input details", func() {
			url := atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/jobs/some-job/builds/%s", teamName, pipelineName, build.Name))

			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page.All(".details li"), 60*time.Second).Should(HaveCount(1))
			Expect(page.Find(".details li:first-child")).To(HaveText(`some-resource-with-no-versions - no versions available`))
		})
	})

	Context("when no versions have passed constraints", func() {
		BeforeEach(func() {
			config := atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get: "some-resource-with-passed-constraints",
							},
						},
					},
					{
						Name: "second-job",
						Plan: atc.PlanSequence{
							{
								Get:    "some-resource-with-passed-constraints",
								Passed: []string{"some-job"},
							},
						},
					},
				},
				Resources: []atc.ResourceConfig{
					{
						Name: "some-resource-with-passed-constraints",
						Type: "git",
						Source: atc.Source{
							"branch": "master",
							"uri":    originGitServer.URI(),
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

			build, err = team.CreateJobBuild(pipelineName, "second-job")
			Expect(err).NotTo(HaveOccurred())
		})

		It("displays input details", func() {
			url := atcRoute(fmt.Sprintf("/teams/%s/pipelines/%s/jobs/second-job/builds/%s", teamName, pipelineName, build.Name))

			Expect(page.Navigate(url)).To(Succeed())
			Eventually(page.All(".details li"), 60*time.Second).Should(HaveCount(1))
			Expect(page.Find(".details li:first-child")).To(HaveText("some-resource-with-passed-constraints - no versions satisfy passed constraints"))
		})
	})
})
