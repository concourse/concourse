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
		triggerJob("some-passing-job")
		watch := flyWatch("some-passing-job")
		Eventually(watch).Should(gbytes.Say("passing job on success"))
		Eventually(watch).Should(gbytes.Say("passing job ensure"))
		Eventually(watch).Should(gbytes.Say("passing job on job success"))
		Eventually(watch).Should(gbytes.Say("passing job on job ensure"))
		Expect(watch).To(gexec.Exit(0))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on failure"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("passing job on job failure"))

		By("performing ensure and on_failure steps on failure")
		triggerJob("some-failing-job")
		watch = flyWatch("some-failing-job")
		Eventually(watch).Should(gbytes.Say("failing job on failure"))
		Eventually(watch).Should(gbytes.Say("failing job ensure"))
		Eventually(watch).Should(gbytes.Say("failing job on job failure"))
		Eventually(watch).Should(gbytes.Say("failing job on job ensure"))
		Expect(watch).To(gexec.Exit(1))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on success"))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("failing job on job success"))

		By("performing ensure and on_failure steps on failure")
		triggerJob("some-aborted-job")
		watch = flyWatch("some-aborted-job")
		watch.Kill()
		Eventually(watch).Should(gbytes.Say("aborted job ensure"))
	})
})
