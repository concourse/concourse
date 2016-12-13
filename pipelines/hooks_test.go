package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing jobs with hooks", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/hooks.yml",
		)
	})

	It("performs hooks under the right conditions", func() {
		By("performing ensure and on_success outputs on success")
		watch := triggerJob("some-passing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("passing job on success"))
		Expect(watch).To(gbytes.Say("passing job ensure"))
		Expect(watch).To(gbytes.Say("passing job on job success"))
		Expect(watch).To(gbytes.Say("passing job on job ensure"))
		Expect(watch).To(gexec.Exit(0))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on job failure"))

		By("performing ensure and on_failure steps on failure")
		watch = triggerJob("some-failing-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("failing job on failure"))
		Expect(watch).To(gbytes.Say("failing job ensure"))
		Expect(watch).To(gbytes.Say("failing job on job failure"))
		Expect(watch).To(gbytes.Say("failing job on job ensure"))
		Expect(watch).To(gexec.Exit(1))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on job success"))

		By("performing ensure steps on abort")
		watch = triggerJob("some-aborted-job")
		Eventually(watch).Should(gbytes.Say("waiting to be aborted"))
		watch.Interrupt()
		<-watch.Exited
		Expect(watch).To(gbytes.Say("aborted job ensure"))
		Expect(watch).To(gexec.Exit(1))
	})
})
