package pipelines_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a pipeline", func() {
	var (
		newPipelineName string
	)

	BeforeEach(func() {
		newPipelineName = fmt.Sprintf("renamed-test-pipeline-%d", GinkgoParallelNode())
		destroyPipeline(newPipelineName)
	})

	It("runs scheduled after pipeline is renamed", func() {
		configurePipeline(
			"-c", "fixtures/simple.yml",
		)

		watch := triggerJob("simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))

		renamePipeline(newPipelineName)

		watch = triggerPipelineJob(newPipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))
	})
})
