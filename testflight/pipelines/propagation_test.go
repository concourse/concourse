package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("A pipeline that propagates resources", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/propagation.yml",
		)
	})

	It("propagates resources via implicit and explicit outputs", func() {
		watch := flyHelper.TriggerJob(pipelineName, "first-job")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))

		watch = flyHelper.TriggerJob(pipelineName, "pushing-job")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))

		watch = flyHelper.TriggerJob(pipelineName, "downstream-job")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))
	})
})
