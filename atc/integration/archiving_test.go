package integration_test

import (
	"github.com/concourse/concourse/atc"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var basicPipelineConfig = []byte(`
---
jobs:
- name: simple
`)

var _ = Describe("ATC Integration Test", func() {
	var (
		client      concourse.Client
		pipelineRef = atc.PipelineRef{Name: "pipeline"}
	)

	JustBeforeEach(func() {
		client = login(atcURL, "test", "test")
	})

	It("can archive pipelines", func() {
		givenAPipeline(client, pipelineRef)

		whenIArchiveIt(client, pipelineRef)

		pipeline := getPipeline(client, pipelineRef)
		Expect(pipeline.Archived).To(BeTrue(), "pipeline was not archived")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("fails when unpausing an archived pipeline", func() {
		givenAPipeline(client, pipelineRef)
		whenIArchiveIt(client, pipelineRef)

		_, err := client.Team("main").UnpausePipeline(pipelineRef)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("action not allowed for an archived pipeline"))
	})

	It("archived pipelines can have their name re-used", func() {
		givenAPipeline(client, pipelineRef)
		whenIArchiveIt(client, pipelineRef)

		_, version, _, _ := client.Team("main").PipelineConfig(pipelineRef)
		client.Team("main").CreateOrUpdatePipelineConfig(pipelineRef, version, basicPipelineConfig, false)

		pipeline := getPipeline(client, pipelineRef)
		Expect(pipeline.Archived).To(BeFalse(), "pipeline is still archived")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("archiving a pipeline results in it being paused", func() {
		givenAPipeline(client, pipelineRef)
		whenIUnpauseIt(client, pipelineRef)

		whenIArchiveIt(client, pipelineRef)

		pipeline := getPipeline(client, pipelineRef)
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("archiving a pipeline purges its config", func() {
		givenAPipeline(client, pipelineRef)

		whenIArchiveIt(client, pipelineRef)

		_, hasConfig := getPipelineConfig(client, pipelineRef)
		Expect(hasConfig).To(BeFalse())
	})
})

func givenAPipeline(client concourse.Client, pipelineRef atc.PipelineRef) {
	_, _, _, err := client.Team("main").CreateOrUpdatePipelineConfig(pipelineRef, "0", basicPipelineConfig, false)
	Expect(err).NotTo(HaveOccurred())
}

func whenIUnpauseIt(client concourse.Client, pipelineRef atc.PipelineRef) {
	_, err := client.Team("main").UnpausePipeline(pipelineRef)
	Expect(err).ToNot(HaveOccurred())
}

func whenIArchiveIt(client concourse.Client, pipelineRef atc.PipelineRef) {
	_, err := client.Team("main").ArchivePipeline(pipelineRef)
	Expect(err).ToNot(HaveOccurred())
}

func getPipeline(client concourse.Client, pipelineRef atc.PipelineRef) atc.Pipeline {
	pipeline, _, err := client.Team("main").Pipeline(pipelineRef)
	Expect(err).ToNot(HaveOccurred())
	return pipeline
}

func getPipelineConfig(client concourse.Client, pipelineRef atc.PipelineRef) (atc.Config, bool) {
	config, _, ok, err := client.Team("main").PipelineConfig(pipelineRef)
	Expect(err).ToNot(HaveOccurred())
	return config, ok
}
