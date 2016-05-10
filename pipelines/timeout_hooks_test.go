package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline containing a job with a timeout and hooks", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/timeout_hooks.yml",
		)
	})

	It("runs the failure and ensure hooks", func() {
		triggerJob("duration-fail-job")
		watch := flyWatch("duration-fail-job")
		Eventually(watch).Should(gbytes.Say("duration fail job on failure"))
		Eventually(watch).Should(gbytes.Say("duration fail job ensure"))
		Eventually(watch).Should(gexec.Exit(1))
		Expect(watch.Out.Contents()).NotTo(ContainSubstring("duration fail job on success"))
	})
})
