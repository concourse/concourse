package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with a try step", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/try.yml")
	})

	It("proceeds through the plan even if the step fails", func() {
		watch := fly("trigger-job", "-j", inPipeline("try-job"), "-w")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("passing-task succeeded"))
	})
})
