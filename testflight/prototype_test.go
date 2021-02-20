package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a prototype in a pipeline config", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	Context("with custom prototypes", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/prototypes.yml", "-v", "hash="+hash)
		})

		It("can use custom prototypes for 'get', 'put', and task 'image_resource's", func() {
			watch := fly("trigger-job", "-j", inPipeline("resource-getter"), "-w")
			Expect(watch).To(gbytes.Say("fetched version: " + hash))

			watch = fly("trigger-job", "-j", inPipeline("resource-putter"), "-w")
			Expect(watch).To(gbytes.Say("pushing version: some-pushed-version"))

			watch = fly("trigger-job", "-j", inPipeline("resource-image-resourcer"), "-w")
			Expect(watch).To(gbytes.Say("MIRRORED_VERSION=image-version"))
		})

		It("can check for resources having a custom type recursively", func() {
			checkResource := fly("check-resource", "-r", inPipeline("my-resource"))
			Expect(checkResource).To(gbytes.Say("custom-resource-type"))
			Expect(checkResource).To(gbytes.Say("succeeded"))
			Expect(checkResource).To(gbytes.Say("my-resource"))
			Expect(checkResource).To(gbytes.Say("succeeded"))
		})

		It("can check for resources having a custom type shallowly and error out", func() {
			checkResource := spawnFly("check-resource", "-r", inPipeline("my-resource"), "--shallow")
			<-checkResource.Exited
			Expect(checkResource).To(gexec.Exit(1))
			Expect(checkResource.Err).To(gbytes.Say("prototype '.*' has no version"))
		})
	})

	// Context("with custom prototypes that have params", func() {
	// 	BeforeEach(func() {
	// 		setAndUnpausePipeline("fixtures/resource-types-with-params.yml", "-v", "hash="+hash)
	// 	})

	// 	It("can use a custom resource with parameters", func() {
	// 		watch := fly("trigger-job", "-j", inPipeline("resource-test"), "-w")
	// 		Expect(watch).To(gbytes.Say(hash))
	// 	})
	// })

	// Context("when prototype named as base prototype", func() {
	// 	BeforeEach(func() {
	// 		setAndUnpausePipeline("fixtures/resource-type-named-as-base-type.yml", "-v", "hash="+hash)
	// 	})

	// 	It("can use custom prototype named as base prototype", func() {
	// 		watch := fly("trigger-job", "-j", inPipeline("resource-getter"), "-w")
	// 		Expect(watch).To(gbytes.Say("mirror-" + hash))
	// 	})
	// })

	// Context("when prototype has defaults", func() {
	// 	BeforeEach(func() {
	// 		setAndUnpausePipeline("fixtures/resource-type-defaults.yml", "-v", "hash="+hash)
	// 	})

	// 	It("applies the defaults for check, get, and put steps", func() {
	// 		check := fly("check-resource", "-r", inPipeline("some-resource"))
	// 		Expect(check).To(gbytes.Say("defaulted"))

	// 		getAndPut := fly("trigger-job", "-j", inPipeline("some-job"), "-w")
	// 		Expect(getAndPut).To(gbytes.Say("defaulted"))
	// 		Expect(getAndPut).To(gbytes.Say("fetching version: " + hash))
	// 		Expect(getAndPut).To(gbytes.Say("defaulted"))
	// 		Expect(getAndPut).To(gbytes.Say("pushing version: put-version"))
	// 		Expect(getAndPut).To(gbytes.Say("defaulted"))
	// 		Expect(getAndPut).To(gbytes.Say("fetching version: put-version"))
	// 	})
	// })
})
