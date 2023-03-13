package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("load_var Step", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/load-var-step.yml")
	})

	It("uses the load_var step build execution", func() {
		fly("trigger-job", "-j", inPipeline("use-var"), "-w")
	})
})
