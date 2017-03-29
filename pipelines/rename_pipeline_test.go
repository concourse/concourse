package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a pipeline", func() {
	It("runs scheduled after pipeline is renamed", func() {
		configurePipeline(
			"-c", "fixtures/simple.yml",
		)

		watch := triggerJob("simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))

		renamePipeline()

		watch = triggerPipelineJob(pipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))
	})
})
