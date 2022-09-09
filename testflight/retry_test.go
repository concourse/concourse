package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a step that retries", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/retry.yml")
	})

	It("retries until the step succeeds", func() {
		watch := fly("trigger-job", "-j", inPipeline("retry-job"), "-w")
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
			watch := spawnFly("trigger-job", "-j", inPipeline("retry-job-fail-for-hijacking"), "-w")
			// wait until job finishes before trying to hijack
			<-watch.Exited
			Expect(watch).To(gexec.Exit(1))
		})

		It("permits hijacking a specific attempt", func() {
			fly(
				"intercept",
				"-j", pipelineName+"/retry-job-fail-for-hijacking",
				"-s", "succeed-on-3rd-attempt",
				"--attempt", "2",
				"--",
				"sh", "-c", "[ `cat /tmp/retry_number` -eq 2 ]",
			)
		})

		It("correctly displays information about attempts", func() {
			hijackS = fly("intercept", "-j", pipelineName+"/retry-job-fail-for-hijacking", "-s", "succeed-on-3rd-attempt", "--", "sh", "-c", "exit")
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gbytes.Say("[1-9]*: build #1, step: succeed-on-3rd-attempt, type: task, attempt: [1-3]"))
			Eventually(hijackS).Should(gexec.Exit())
		})
	})
})
