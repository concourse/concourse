package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("When a resource type depends on another resource type", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/recursive-resource-checking.yml")
	})

	It("can be checked recursively", func() {
		check := fly("check-resource", "-r", inPipeline("recursive-custom-resource"))
		Expect(check).To(gbytes.Say("recursive-custom-resource"))
		Expect(check).To(gbytes.Say("mock-resource-grandchild"))
		Expect(check).To(gbytes.Say("mock-resource-child"))
		Expect(check).To(gbytes.Say("mock-resource-parent"))
		Expect(check).To(gbytes.Say("succeeded"))
	})
})
