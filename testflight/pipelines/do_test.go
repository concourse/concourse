package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A pipeline containing a do", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/do.yml",
		)
	})

	It("performs the do steps", func() {
		watch := flyHelper.TriggerJob(pipelineName, "do-job")
		<-watch.Exited

		By("running the first step")
		Expect(watch).To(gbytes.Say("running do step 1"))

		By("running the second step")
		Expect(watch).To(gbytes.Say("running do step 2"))

		By("running the third step")
		Expect(watch).To(gbytes.Say("running do step 3"))
	})
})
