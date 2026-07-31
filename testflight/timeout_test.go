package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task with a timeout", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/timeout.yml")
	})

	It("enforces the timeout", func(ctx SpecContext) {
		successWatch := spawnFly("trigger-job", "-j", inPipeline("duration-successful-job"), "-w")
		failedWatch := spawnFly("trigger-job", "-j", inPipeline("duration-fail-job"), "-w")

		By("not aborting if the step completes in time")
		Eventually(successWatch).Should(gexec.Exit(0))
		Expect(successWatch).To(gbytes.Say("initializing"))
		Expect(successWatch).To(gbytes.Say("passing-task succeeded"))

		By("aborting when the step takes too long")
		Eventually(failedWatch).Should(gexec.Exit(1))
		Expect(failedWatch).To(gbytes.Say("initializing"))
		Expect(failedWatch).To(gbytes.Say("timeout exceeded"))
	}, DefaultSpecTimeout)
})
