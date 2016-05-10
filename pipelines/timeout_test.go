package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task with a timeout", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/timeout.yml",
		)
	})

	It("enforces the timeout", func() {
		triggerJob("duration-successful-job")
		triggerJob("duration-fail-job")

		By("not aborting if the step completes in time")
		watch := flyWatch("duration-successful-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("passing-task succeeded"))
		Expect(watch).To(gexec.Exit(0))

		By("aborting when the step takes too long")
		watch = flyWatch("duration-fail-job")
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("interrupted"))
		Expect(watch).To(gexec.Exit(1))
	})
})
