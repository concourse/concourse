package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline containing a do", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/do.yml",
		)
	})

	It("performs the do steps", func() {
		triggerJob("do-job")
		watch := flyWatch("do-job")

		By("running the first step")
		Eventually(watch).Should(gbytes.Say("running do step 1"))

		By("running the second step")
		Eventually(watch).Should(gbytes.Say("running do step 2"))

		By("running the third step")
		Eventually(watch).Should(gbytes.Say("running do step 3"))
	})
})
