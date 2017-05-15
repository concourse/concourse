package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task with a timeout", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/timeout.yml",
		)
	})

	It("enforces the timeout", func() {
		successWatch := flyHelper.TriggerJob(pipelineName, "duration-successful-job")
		failedWatch := flyHelper.TriggerJob(pipelineName, "duration-fail-job")

		By("not aborting if the step completes in time")
		<-successWatch.Exited
		Expect(successWatch).To(gbytes.Say("initializing"))
		Expect(successWatch).To(gbytes.Say("passing-task succeeded"))
		Expect(successWatch).To(gexec.Exit(0))

		By("aborting when the step takes too long")
		<-failedWatch.Exited
		Expect(failedWatch).To(gbytes.Say("initializing"))
		Expect(failedWatch).To(gbytes.Say("interrupted"))
		Expect(failedWatch).To(gexec.Exit(1))
	})
})
