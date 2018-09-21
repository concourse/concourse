package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = FDescribe("A job with an input with trigger: true", func() {
	BeforeEach(func() {
		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/simple-trigger.yml",
		)
	})

	It("triggers when the resource changes", func() {
		By("running on the initial version")
		flyHelper.CheckResource("-r", pipelineName+"/some-resource", "-f", "version:first-version")
		watch := flyHelper.Watch(pipelineName, "some-passing-job")
		Eventually(watch).Should(gbytes.Say("first-version"))

		By("building another commit")
		flyHelper.CheckResource("-r", pipelineName+"/some-resource", "-f", "version:second-version")
		watch = flyHelper.Watch(pipelineName, "some-passing-job", "2")
		Eventually(watch).Should(gbytes.Say("second-version"))
	})
})
