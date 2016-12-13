package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a try step", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/try.yml",
		)
	})

	It("proceeds through the plan even if the step fails", func() {
		watch := triggerJob("try-job")
		<-watch.Exited
		Expect(watch).To(gbytes.Say("initializing"))
		Expect(watch).To(gbytes.Say("passing-task succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
