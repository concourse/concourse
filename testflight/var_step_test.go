package testflight_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Var Step", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/var-step.yml")
	})

	It("uses the var step build execution", func() {
		fly("trigger-job", "-j", inPipeline("use-var"), "-w")
	})
})
