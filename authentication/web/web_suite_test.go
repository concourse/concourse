package web_test

import (
	"fmt"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/concourse/testflight/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sclevine/agouti"

	"testing"
)

var atcURL = helpers.AtcURL()
var pipelineName string
var teamName string
var publicBuild, privateBuild atc.Build
var brokenResource atc.Resource

var agoutiDriver *agouti.WebDriver
var page *agouti.Page

var client concourse.Client
var team concourse.Team

var _ = SynchronizedBeforeSuite(func() []byte {
	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	data, err := helpers.FirstNodeClientSetup(atcURL)
	Expect(err).NotTo(HaveOccurred())

	return data
}, func(data []byte) {
	var err error
	client, err = helpers.AllNodeClientSetup(data)
	Expect(err).NotTo(HaveOccurred())

	team = client.Team("main")

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())
	teamName = "main"

	agoutiDriver = helpers.AgoutiDriver()
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	_, err := team.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())

	pushMainPipeline()

	page, err = agoutiDriver.NewPage()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	Expect(page.Destroy()).To(Succeed())

	err := helpers.DeleteAllContainers(client, pipelineName)
	Expect(err).ToNot(HaveOccurred())

	_, err = team.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
})

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authentication Web Suite")
}

func pushMainPipeline() {
	_, _, _, err := team.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
		Jobs: []atc.JobConfig{
			{
				Name:   "public-job",
				Public: true,
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
								Args: []string{"-c", "sleep 30 && echo public job info"},
							},
						},
					},
				},
			},
			{
				Name:   "private-job",
				Public: false,
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
								Path: "echo",
								Args: []string{"private job info"},
							},
						},
					},
				},
			},
		},
		Resources: []atc.ResourceConfig{
			{
				Name: "broken-resource",
				Type: "git",
				Source: atc.Source{
					"branch": "master",
					"uri":    "git@github.com:concourse/deployments.git",
				},
				CheckEvery: "",
			},
		},
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = team.RevealPipeline(pipelineName)
	Expect(err).NotTo(HaveOccurred())

	_, err = team.UnpausePipeline(pipelineName)
	Expect(err).NotTo(HaveOccurred())

	publicBuild, err = team.CreateJobBuild(pipelineName, "public-job")
	Expect(err).NotTo(HaveOccurred())

	privateBuild, err = team.CreateJobBuild(pipelineName, "private-job")
	Expect(err).NotTo(HaveOccurred())

	var found bool
	brokenResource, found, err = team.Resource(pipelineName, "broken-resource")
	Expect(found).To(BeTrue())
	Expect(err).NotTo(HaveOccurred())
}
