package pipelines_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	Context("with custom resource types", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types.yml",
			)
		})

		It("can use custom resource types for 'get', 'put', and task 'image_resource's", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("fetched version: hello-from-custom-type"))

			watch = flyHelper.TriggerJob(pipelineName, "resource-putter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("pushing version: some-pushed-version"))

			watch = flyHelper.TriggerJob(pipelineName, "resource-image-resourcer")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("MIRRORED_VERSION=image-version"))
		})

		It("can check for resources using a custom type", func() {
			checkResource := flyHelper.CheckResource("-r", fmt.Sprintf("%s/my-resource", pipelineName))
			<-checkResource.Exited
			Expect(checkResource.ExitCode()).To(Equal(0))
			Expect(checkResource).To(gbytes.Say("checked 'my-resource'"))
		})
	})

	Context("with custom resource types that have params", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-types-with-params.yml",
			)
		})

		It("can use a custom resource with parameters", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-test")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("mock"))
		})
	})

	Context("when resource type named as base resource type", func() {
		BeforeEach(func() {
			flyHelper.ConfigurePipeline(
				pipelineName,
				"-c", "fixtures/resource-type-named-as-base-type.yml",
			)
		})

		It("can use custom resource type named as base resource type", func() {
			watch := flyHelper.TriggerJob(pipelineName, "resource-getter")
			<-watch.Exited
			Expect(watch.ExitCode()).To(Equal(0))
			Expect(watch).To(gbytes.Say("mirror-mirror"))
		})
	})
})
