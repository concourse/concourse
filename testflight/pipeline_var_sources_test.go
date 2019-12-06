package testflight_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Pipeline Var Sources", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/var-sources.yml")
	})

	It("uses the pipeline var sources for resource checking and build execution", func() {
		fly("trigger-job", "-j", inPipeline("use-vars"), "-w")
	})
})
