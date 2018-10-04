package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task with a timeout", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/timeout.yml")
	})

	It("enforces the timeout", func() {
		successWatch := spawnFly("trigger-job", "-j", inPipeline("duration-successful-job"), "-w")
		failedWatch := spawnFly("trigger-job", "-j", inPipeline("duration-fail-job"), "-w")

		By("not aborting if the step completes in time")
		<-successWatch.Exited
		Expect(successWatch).To(gbytes.Say("initializing"))
		Expect(successWatch).To(gbytes.Say("passing-task succeeded"))
		Expect(successWatch).To(gexec.Exit(0))

		By("aborting when the step takes too long")
		<-failedWatch.Exited
		Expect(failedWatch).To(gbytes.Say("initializing"))
		Expect(failedWatch).To(gbytes.Say("timeout exceeded"))
		Expect(failedWatch).To(gexec.Exit(1))
	})
})
