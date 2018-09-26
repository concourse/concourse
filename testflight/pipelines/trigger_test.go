package pipelines_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("A job with an input with trigger: true", func() {
	BeforeEach(func() {
		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		flyHelper.ConfigurePipeline(
			pipelineName,
			"-c", "fixtures/simple-trigger.yml",
			"-v", "hash="+hash.String(),
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
