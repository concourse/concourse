package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A task with no outputs declared", func() {
	BeforeEach(func() {
		configurePipeline(
			"-c", "fixtures/task-missing-outputs.yml",
		)
	})

	It("doesn't mount its file system into the next task", func() {
		triggerJob("missing-outputs-job")
		watch := flyWatch("missing-outputs-job")
		Expect(watch).To(gexec.Exit(2))
		Expect(watch).To(gbytes.Say("missing inputs: missing-outputs"))
	})
})
