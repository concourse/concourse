package integration_test

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/concourse/concourse/atc"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("ATC Integration Test", func() {
	var (
		atcProcess ifrit.Process
		atcURL     string
	)

	BeforeEach(func() {
		cmd.EnableArchivePipeline = true
	})

	JustBeforeEach(func() {
		atcURL = fmt.Sprintf("http://localhost:%v", cmd.BindPort)
		runner, err := cmd.Runner([]string{})
		Expect(err).NotTo(HaveOccurred())

		atcProcess = ifrit.Invoke(runner)

		Eventually(func() error {
			_, err := http.Get(atcURL + "/api/v1/info")
			return err
		}, 20*time.Second).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		atcProcess.Signal(os.Interrupt)
		<-atcProcess.Wait()
	})

	It("can archive pipelines", func() {
		client := login(atcURL, "test", "test")
		givenAPipeline(client, "pipeline")
		whenIArchiveIt(client, "pipeline")
		pipeline := getPipeline(client, "pipeline")
		Expect(pipeline.Archived).To(BeTrue(), "pipeline was not archived")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("fails when unpausing an archived pipeline", func() {
		client := login(atcURL, "test", "test")
		givenAPipeline(client, "pipeline")
		whenIArchiveIt(client, "pipeline")
		_, err := client.Team("main").UnpausePipeline("pipeline")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("archived pipelines cannot be unpaused"))
	})

	Context("when the archiving pipeline endpoint is not enabled", func() {
		BeforeEach(func() {
			cmd.EnableArchivePipeline = false
		})

		It("returns an error", func() {
			client := login(atcURL, "test", "test")
			givenAPipeline(client, "pipeline")
			response := whenIArchiveIt(client, "pipeline")
			Expect(response.StatusCode).To(Equal(http.StatusForbidden))
		})
	})
})

func givenAPipeline(client concourse.Client, pipelineName string) {
	config := []byte(`
---
jobs:
- name: simple
`)
	_, _, _, err := client.Team("main").CreateOrUpdatePipelineConfig(pipelineName, "0", config, false)
	Expect(err).NotTo(HaveOccurred())
}

func whenIArchiveIt(client concourse.Client, pipelineName string) *http.Response {
	httpClient := client.HTTPClient()
	request, _ := http.NewRequest(
		"PUT",
		client.URL()+"/api/v1/teams/main/pipelines/"+pipelineName+"/archive",
		nil,
	)
	response, err := httpClient.Do(request)
	Expect(err).ToNot(HaveOccurred())
	return response
}

func getPipeline(client concourse.Client, pipelineName string) atc.Pipeline {
	pipeline, _, err := client.Team("main").Pipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
	return pipeline
}
