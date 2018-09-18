package pipelines_test

import (
	"fmt"

	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a pipeline", func() {
	It("runs scheduled after pipeline is renamed", func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/simple.yml",
		)

		watch := flyHelper.TriggerJob(pipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))

		guid, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		newPipelineName := fmt.Sprintf("test-pipeline-%d-renamed-%s", GinkgoParallelNode(), guid)

		flyHelper.RenamePipeline(pipelineName, newPipelineName)
		pipelineName = newPipelineName

		watch = flyHelper.TriggerPipelineJob(pipelineName, "simple")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("Hello, world!"))
	})
})
