package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing jobs with hooks", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/hooks.yml")
	})

	It("performs hooks under the right conditions", func() {
		By("performing ensure and on_success outputs on success")
		watch := spawnFly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("passing job on success"))
		Expect(watch).To(gbytes.Say("passing job ensure"))
		Expect(watch).To(gbytes.Say("passing job on job success"))
		Expect(watch).To(gbytes.Say("passing job on job ensure"))
		Expect(watch).To(gexec.Exit(0))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on job failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on abort"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on job abort"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on error"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on job error"))

		By("performing ensure and on_failure steps on failure")
		watch = spawnFly("trigger-job", "-j", inPipeline("some-failing-job"), "-w")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("failing job on failure"))
		Expect(watch).To(gbytes.Say("failing job ensure"))
		Expect(watch).To(gbytes.Say("failing job on job failure"))
		Expect(watch).To(gbytes.Say("failing job on job ensure"))
		Expect(watch).To(gexec.Exit(1))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on job success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on abort"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on job abort"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on error"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on job error"))

		By("performing ensure and on_abort steps on abort")
		watch = spawnFly("trigger-job", "-j", inPipeline("some-aborted-job"), "-w")
		// The first eventually is for the initialization block, when the script hasn't been executed yet.
		Eventually(watch).Should(gbytes.Say("echo waiting to be aborted"))
		Eventually(watch).Should(gbytes.Say("waiting to be aborted"))
		fly("abort-build", "-j", inPipeline("some-aborted-job"), "-b", "1")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("aborted job on abort"))
		Expect(watch).To(gbytes.Say("aborted job ensure"))
		Expect(watch).To(gbytes.Say("aborted job on job abort"))
		Expect(watch).To(gbytes.Say("aborted job on job ensure"))
		Expect(watch).To(gexec.Exit(3))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on job success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on job failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on error"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("aborted job on job error"))

		By("performing ensure and on_error steps on error")
		watch = spawnFly("trigger-job", "-j", inPipeline("some-errored-job"), "-w")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("errored job on error"))
		Expect(watch).To(gbytes.Say("errored job ensure"))
		Expect(watch).To(gbytes.Say("errored job on job error"))
		Expect(watch).To(gbytes.Say("errored job on job ensure"))
		Expect(watch).To(gexec.Exit(2))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on job success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on job failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on abort"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("errored job on job abort"))
	})
})
