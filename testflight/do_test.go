package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline containing a do", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/do.yml")
	})

	It("performs the do steps", func() {
		watch := fly("trigger-job", "-j", inPipeline("do-job"), "-w")

		By("running the first step")
		Expect(watch).To(gbytes.Say("running do step 1"))

		By("running the second step")
		Expect(watch).To(gbytes.Say("running do step 2"))

		By("running the third step")
		Expect(watch).To(gbytes.Say("running do step 3"))
	})
})
