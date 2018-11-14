package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("image resource caching", func() {
	var version string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		version = u.String()

		hash, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/image-resource-with-params.yml",
			"-v", "initial_version="+version,
			"-v", "hash="+hash.String(),
		)
	})

	It("gets the image resource from the cache based on given params", func() {
		By("triggering the resource get with params")
		watch := fly("trigger-job", "-j", inPipeline("with-params"), "-w")
		Expect(watch).To(gbytes.Say(`fetching.*` + version))

		By("triggering the task with image resource with params")
		watch = fly("trigger-job", "-j", inPipeline("image-resource-with-params"), "-w")
		Expect(watch).ToNot(gbytes.Say("fetching"))

		By("triggering the task with image resource without params")
		watch = fly("trigger-job", "-j", inPipeline("image-resource-without-params"), "-w")
		Expect(watch).To(gbytes.Say(`fetching.*` + version))

		By("triggering the resource get without params")
		watch = fly("trigger-job", "-j", inPipeline("without-params"), "-w")
		Expect(watch).ToNot(gbytes.Say(`fetching.*` + version))
	})
})
