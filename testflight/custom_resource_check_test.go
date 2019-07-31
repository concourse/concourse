package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("When a resource type depends on another resource type", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/recursive-resource-checking.yml")
	})

	It("errors when parent has no version", func() {
		check := spawnFly("check-resource", "-r", inPipeline("recursive-custom-resource"), "-w")
		<-check.Exited
		Expect(check).To(gexec.Exit(1))
		Expect(check.Err).To(gbytes.Say("parent type has no version"))
	})

	It("can be checked in order", func() {
		check := fly("check-resource-type", "-r", inPipeline("mock-resource-parent"), "-w")
		Expect(check).To(gbytes.Say("mock-resource-parent.*succeeded"))

		check = fly("check-resource-type", "-r", inPipeline("mock-resource-child"), "-w")
		Expect(check).To(gbytes.Say("mock-resource-child.*succeeded"))

		check = fly("check-resource-type", "-r", inPipeline("mock-resource-grandchild"), "-w")
		Expect(check).To(gbytes.Say("mock-resource-grandchild.*succeeded"))

		check = fly("check-resource", "-r", inPipeline("recursive-custom-resource"), "-w")
		Expect(check).To(gbytes.Say("recursive-custom-resource.*succeeded"))
	})

	It("can be checked recursively", func() {
		check := fly("check-resource", "-r", inPipeline("recursive-custom-resource"), "-w", "--recursive")
		Expect(check).To(gbytes.Say("mock-resource-parent.*succeeded"))
		Expect(check).To(gbytes.Say("mock-resource-child.*succeeded"))
		Expect(check).To(gbytes.Say("mock-resource-grandchild.*succeeded"))
		Expect(check).To(gbytes.Say("recursive-custom-resource.*succeeded"))
	})
})
