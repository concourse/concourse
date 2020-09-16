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

	It("can be checked shallowly and errors when parent has no version", func() {
		check := spawnFly("check-resource", "-r", inPipeline("recursive-custom-resource"), "--shallow")
		<-check.Exited
		Expect(check).To(gexec.Exit(1))
		Expect(check.Err).To(gbytes.Say("resource type '.*' has no version"))
	})

	It("can be checked recursively", func() {
		check := fly("check-resource", "-r", inPipeline("recursive-custom-resource"))
		Expect(check).To(gbytes.Say("mock-resource-parent.*succeeded"))
		Expect(check).To(gbytes.Say("mock-resource-child.*succeeded"))
		Expect(check).To(gbytes.Say("mock-resource-grandchild.*succeeded"))
		Expect(check).To(gbytes.Say("recursive-custom-resource.*succeeded"))
	})
})
