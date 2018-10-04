package testflight_test

import (
	. "github.com/onsi/ginkgo"
)

var _ = Describe("A pipeline that propagates resources", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/propagation.yml")
	})

	It("propagates resources via implicit and explicit outputs", func() {
		fly("trigger-job", "-j", inPipeline("first-job"), "-w")
		fly("trigger-job", "-j", inPipeline("pushing-job"), "-w")
		fly("trigger-job", "-j", inPipeline("downstream-job"), "-w")
	})
})
