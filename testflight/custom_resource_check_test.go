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
		Expect(check.Out).To(gbytes.Say("errored"))
	})

	It("can be checked in order", func() {
		check := fly("check-resource-type", "-r", inPipeline("mock-resource-parent"), "-w")
		Expect(check).To(gbytes.Say("succeeded"))

		check = fly("check-resource-type", "-r", inPipeline("mock-resource-child"), "-w")
		Expect(check).To(gbytes.Say("succeeded"))

		check = fly("check-resource-type", "-r", inPipeline("mock-resource-grandchild"), "-w")
		Expect(check).To(gbytes.Say("succeeded"))

		check = fly("check-resource", "-r", inPipeline("recursive-custom-resource"), "-w")
		Expect(check).To(gbytes.Say("succeeded"))
	})
})
