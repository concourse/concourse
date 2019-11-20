package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline that propagates resources", func() {
	Context("when the inputs and outputs are different resources", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/propagation.yml")
		})

		It("propagates resources via implicit and explicit outputs", func() {
			fly("trigger-job", "-j", inPipeline("first-job"), "-w")
			fly("trigger-job", "-j", inPipeline("pushing-job"), "-w")
			fly("trigger-job", "-j", inPipeline("downstream-job"), "-w")
		})
	})

	Context("when the input/output are the same resource", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/inputs_outputs.yml")
		})

		It("propogates the output version over the input version", func() {
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:first-version")
			fly("check-resource", "-r", inPipeline("some-resource"), "-f", "version:second-version")

			watch := fly("trigger-job", "-j", inPipeline("pushing-job"), "-w")
			Expect(watch).To(gbytes.Say("succeeded"))

			watch = fly("trigger-job", "-j", inPipeline("downstream-job"), "-w")
			Expect(watch).ToNot(gbytes.Say("second-version"))
			Expect(watch).To(gbytes.Say("first-version"))
		})
	})
})
