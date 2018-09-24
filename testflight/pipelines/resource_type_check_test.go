package pipelines_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource-types checks", func() {
	Context("Updating resource types", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
			)
		})

		XIt("uses updated resource type", func() {
			By("watching for first resource-imgur")
			watch := flyHelper.Watch(pipelineName, "resource-imgur", "1")
			Expect(watch).To(gbytes.Say("fetched from custom resource"))
			Expect(watch).To(gexec.Exit(0))

			Skip("need support for check-resource-type from version")
		})
	})

	Context("check-resource-type", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
			)
		})

		It("can check the resource-type", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/custom-resource-type")
			Eventually(watch).Should(gbytes.Say("checked 'custom-resource-type'"))
			Eventually(watch).Should(gexec.Exit(0))
		})

		It("reports that resource-type is not found if it doesn't exist", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/nonexistent-resource-type")
			Eventually(watch.Err).Should(gbytes.Say("resource-type 'nonexistent-resource-type' not found"))
			Eventually(watch).Should(gexec.Exit(1))
		})

		It("fails when resource-type check fails", func() {
			watch := flyHelper.CheckResourceType("-r", pipelineName+"/failing-custom-resource-type")
			Eventually(watch.Err).Should(gbytes.Say("check failed"))
			Eventually(watch.Err).Should(gbytes.Say("im totally failing to check"))
			Eventually(watch).Should(gexec.Exit(1))
		})
	})
})
