package integration_test

import (
	"github.com/concourse/concourse/atc"
	concourse "github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var basicPipelineConfig = []byte(`
---
jobs:
- name: simple
`)

var _ = Describe("ATC Integration Test", func() {
	var (
		client concourse.Client
	)

	JustBeforeEach(func() {
		client = login(atcURL, "test", "test")
	})

	It("can archive pipelines", func() {
		givenAPipeline(client, "pipeline")

		whenIArchiveIt(client, "pipeline")

		pipeline := getPipeline(client, "pipeline")
		Expect(pipeline.Archived).To(BeTrue(), "pipeline was not archived")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("fails when unpausing an archived pipeline", func() {
		givenAPipeline(client, "pipeline")
		whenIArchiveIt(client, "pipeline")

		_, err := client.Team("main").UnpausePipeline("pipeline")

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("action not allowed for an archived pipeline"))
	})

	It("archived pipelines can have their name re-used", func() {
		givenAPipeline(client, "pipeline")
		whenIArchiveIt(client, "pipeline")

		_, version, _, _ := client.Team("main").PipelineConfig("pipeline")
		client.Team("main").CreateOrUpdatePipelineConfig("pipeline", version, basicPipelineConfig, false)

		pipeline := getPipeline(client, "pipeline")
		Expect(pipeline.Archived).To(BeFalse(), "pipeline is still archived")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("archiving a pipeline results in it being paused", func() {
		givenAPipeline(client, "pipeline")
		whenIUnpauseIt(client, "pipeline")

		whenIArchiveIt(client, "pipeline")

		pipeline := getPipeline(client, "pipeline")
		Expect(pipeline.Paused).To(BeTrue(), "pipeline was not paused")
	})

	It("archiving a pipeline purges its config", func() {
		givenAPipeline(client, "pipeline")

		whenIArchiveIt(client, "pipeline")

		_, hasConfig := getPipelineConfig(client, "pipeline")
		Expect(hasConfig).To(BeFalse())
	})

	Context("when the archiving pipeline endpoint is not enabled", func() {
		It("returns an error", func() {
			givenAPipeline(client, "pipeline")

			_, err := client.Team("main").ArchivePipeline("pipeline")

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("forbidden"))
		})
	})
})

func givenAPipeline(client concourse.Client, pipelineName string) {
	_, _, _, err := client.Team("main").CreateOrUpdatePipelineConfig(pipelineName, "0", basicPipelineConfig, false)
	Expect(err).NotTo(HaveOccurred())
}

func whenIUnpauseIt(client concourse.Client, pipelineName string) {
	_, err := client.Team("main").UnpausePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
}

func whenIArchiveIt(client concourse.Client, pipelineName string) {
	_, err := client.Team("main").ArchivePipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
}

func getPipeline(client concourse.Client, pipelineName string) atc.Pipeline {
	pipeline, _, err := client.Team("main").Pipeline(pipelineName)
	Expect(err).ToNot(HaveOccurred())
	return pipeline
}

func getPipelineConfig(client concourse.Client, pipelineName string) (atc.Config, bool) {
	config, _, ok, err := client.Team("main").PipelineConfig(pipelineName)
	Expect(err).ToNot(HaveOccurred())
	return config, ok
}
