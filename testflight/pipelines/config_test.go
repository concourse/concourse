package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resource with nested configuration", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/nested-config-test.yml",
		)
	})

	It("works", func() {
		watch := flyHelper.TriggerJob(pipelineName, "config-test")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))
	})
})
