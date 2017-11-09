package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = XDescribe("A task using a custom resource as image_resource", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/custom-resource-type-as-image-resource.yml",
			"-y", "privileged=true",
		)
	})

	It("sucessfully runs the job when initially triggered", func() {
		watch := flyHelper.TriggerJob(pipelineName, "task-using-custom-type")
		<-watch.Exited
		Expect(watch).To(gexec.Exit(0))
		Expect(watch).To(gbytes.Say("hello world"))
	})
})
