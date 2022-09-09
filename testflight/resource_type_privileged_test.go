package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Configuring a resource type in a pipeline config", func() {
	var privileged string

	BeforeEach(func() {
		privileged = "nil"
	})

	JustBeforeEach(func() {
		unique, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-types-privileged.yml",
			"-y", "privileged="+privileged,
			"-v", "unique_config="+unique.String(),
		)
	})

	Context("when the resource type is privileged", func() {
		BeforeEach(func() {
			privileged = "true"
		})

		It("performs 'check', 'get', and 'put' with privileged containers", func() {
			By("running 'get' with a privileged container")
			watch := fly("trigger-job", "-j", inPipeline("resource-getter"), "-w")
			Expect(watch).To(gbytes.Say("privileged: true"))

			By("running the resource 'check' with a privileged container")
			versions := fly("resource-versions", "-r", inPipeline("my-resource"))
			Expect(versions).To(gbytes.Say("privileged:true,version:mock"))

			By("running 'put' with a privileged container")
			watch = fly("trigger-job", "-j", inPipeline("resource-putter"), "-w")
			Expect(watch).To(gbytes.Say("pushing in a privileged container"))
		})
	})

	Context("when the custom resource type is not privileged", func() {
		BeforeEach(func() {
			privileged = "false"
		})

		It("performs 'check', 'get', and 'put' with unprivileged containers", func() {
			By("running 'get' with an unprivileged container")
			watch := fly("trigger-job", "-j", inPipeline("resource-getter"), "-w")
			Expect(watch).To(gbytes.Say("privileged: false"))

			By("running the resource 'check' with an unprivileged container")
			versions := fly("resource-versions", "-r", inPipeline("my-resource"))
			Expect(versions).ToNot(gbytes.Say("privileged:true"))

			By("running 'put' with an unprivileged container")
			watch = fly("trigger-job", "-j", inPipeline("resource-putter"), "-w")
			Expect(watch).ToNot(gbytes.Say("running in a privileged container"))
		})
	})
})
