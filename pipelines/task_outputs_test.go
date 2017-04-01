package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A job with a task that produces outputs", func() {
	Context("with outputs and single worker", func() {
		BeforeEach(func() {
			configurePipeline(
				"-c", "fixtures/task-outputs.yml",
			)
		})

		It("propagates the outputs from one task to another", func() {
			watch := triggerJob("some-job")
			<-watch.Exited
			Expect(watch).To(gbytes.Say("initializing"))
			Expect(watch).To(gexec.Exit(0))

			Expect(watch.Out.Contents()).To(ContainSubstring("./output-1/file-1"))
			Expect(watch.Out.Contents()).To(ContainSubstring("./output-2/file-2"))
		})
	})
})
