package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a step that retries", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(pipelineName, "-c", "fixtures/retry.yml")
	})

	It("retries until the step succeeds", func() {
		watch := flyHelper.TriggerJob(pipelineName, "retry-job")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))

		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("attempts: 1; failing"))
		Expect(watch).To(gbytes.Say("attempts: 2; failing"))
		Expect(watch).To(gbytes.Say("attempts: 3; success!"))
		Expect(watch).ToNot(gbytes.Say("attempts:"))
		Expect(watch).To(gbytes.Say("succeeded"))
	})

	Context("hijacking jobs with retry steps", func() {
		var hijackS *gexec.Session

		BeforeEach(func() {
			watch := flyHelper.TriggerJob(pipelineName, "retry-job-fail-for-hijacking")
			// wait until job finishes before trying to hijack
			<-watch.Exited
			Expect(watch).To(gexec.Exit(1))
		})

		It("permits hijacking a specific attempt", func() {
			hijackS = flyHelper.Hijack(
				"-j", pipelineName+"/retry-job-fail-for-hijacking",
				"-s", "succeed-on-3rd-attempt",
				"--attempt", "2",
				"--", "sh", "-c",
				"[ `cat /tmp/retry_number` -eq 2 ]")
			Eventually(hijackS).Should(gexec.Exit(0))
		})

		It("correctly displays information about attempts", func() {
			hijackS = flyHelper.Hijack("-j", pipelineName+"/retry-job-fail-for-hijacking", "-s", "succeed-on-3rd-attempt", "--", "sh", "-c", "exit")
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gexec.Exit())
		})
	})
})
