package web_test

import (
	"fmt"
	"time"

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
var publicBuild, privateBuild atc.Build

var agoutiDriver *agouti.WebDriver
var page *agouti.Page

var client concourse.Client

var _ = BeforeSuite(func() {
	// observed jobs taking ~1m30s, so set the timeout pretty high
	SetDefaultEventuallyTimeout(1 * time.Minute)

	// poll less frequently
	SetDefaultEventuallyPollingInterval(time.Second)

	Eventually(helpers.ErrorPolling(atcURL)).ShouldNot(HaveOccurred())

	var err error
	client, err = helpers.ConcourseClient(atcURL)
	Expect(err).NotTo(HaveOccurred())

	pipelineName = fmt.Sprintf("test-pipeline-%d", GinkgoParallelNode())

	agoutiDriver = agouti.PhantomJS(agouti.Debug)
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})

var _ = BeforeEach(func() {
	_, err := client.DeletePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
	pushMainPipeline()

	page, err = agoutiDriver.NewPage()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterEach(func() {
	Expect(page.Destroy()).To(Succeed())
})

func TestWeb(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Authentication Web Suite")
}

func pushMainPipeline() {
	_, _, err := client.CreateOrUpdatePipelineConfig(pipelineName, "0", atc.Config{
		Jobs: []atc.JobConfig{
			{
				Name:   "public-job",
				Public: true,
				Plan: atc.PlanSequence{
					{
						Task: "some-task",
						TaskConfig: &atc.TaskConfig{
							Run: atc.TaskRunConfig{
								Path: "echo",
								Args: []string{"public job info"},
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
							Run: atc.TaskRunConfig{
								Path: "echo",
								Args: []string{"private job info"},
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

	publicBuild, err = client.CreateJobBuild(pipelineName, "public-job")
	Expect(err).NotTo(HaveOccurred())

	privateBuild, err = client.CreateJobBuild(pipelineName, "private-job")
	Expect(err).NotTo(HaveOccurred())
}
