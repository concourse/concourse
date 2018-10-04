package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a task that produces outputs", func() {
	Context("with outputs and single worker", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/task-outputs.yml")
		})

		It("propagates the outputs from one task to another", func() {
			watch := fly("trigger-job", "-j", inPipeline("some-job"), "-w")
			Expect(watch).To(gbytes.Say("initializing"))

			Expect(watch.Out.Contents()).To(ContainSubstring("./output-1/file-1"))
			Expect(watch.Out.Contents()).To(ContainSubstring("./output-2/file-2"))
		})
	})
})
