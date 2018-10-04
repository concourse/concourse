package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("When a resource type depends on another resource type", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/recursive-resource-checking.yml")
	})

	It("can be checked immediately", func() {
		checkS := fly("check-resource", "-r", inPipeline("recursive-custom-resource"))
		Expect(checkS).To(gbytes.Say("checked 'recursive-custom-resource'"))
	})
})
