package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Pipeline with prototypes", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/prototype.yml")
	})

	It("executes the build plan correctly", func(ctx SpecContext) {
		watch := fly("trigger-job", "-j", inPipeline("job"), "-w")

		// XXX(prototypes): eventually, this'll need to test the real implementation
		Expect(watch).To(gbytes.Say("run some-message on prototype some-prototype"))
	}, DefaultSpecTimeout)
})
