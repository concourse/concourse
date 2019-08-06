package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Renaming a job", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/simple.yml")
	})

	It("retains job history", func() {
		fly("trigger-job", "-j", inPipeline("simple"), "-w")
		build := fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/simple"))

		setPipeline("fixtures/rename-simple.yml")

		build = fly("builds", "-p", pipelineName)
		Expect(build).To(gbytes.Say(pipelineName + "/rename-simple"))
	})
})
