package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a pipeline", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/simple.yml")
	})

	It("runs scheduled after pipeline is renamed", func() {
		watch := fly("trigger-job", "-j", inPipeline("simple"), "-w")
		Expect(watch).To(gbytes.Say("Hello, world!"))

		newPipelineName := randomPipelineName()

		fly("rename-pipeline", "-o", pipelineName, "-n", newPipelineName)
		pipelineName = newPipelineName

		watch = fly("trigger-job", "-j", inPipeline("simple"), "-w")
		Expect(watch).To(gbytes.Say("Hello, world!"))
	})
})
