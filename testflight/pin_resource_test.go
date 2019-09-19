package testflight_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Pin a resource", func() {
	var (
		someResourceV1, someResourceV2 string
	)
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/resource-with-versions.yml")
	})

	Context("when the resource has partially matching version", func() {
		BeforeEach(func() {
			someResourceV1 = newMockVersion("some-resource", "foo")
			someResourceV2 = newMockVersion("some-resource", "bar")
		})

		It("can pin the resource with given version and comment", func() {
			watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			Expect(watch).To(gbytes.Say("fetching.*" + someResourceV2))
			Expect(watch).To(gbytes.Say("succeeded"))

			watch = fly("pin-resource", "-r", inPipeline("some-resource"), "-v", fmt.Sprintf(`version:%s`, someResourceV1), "-c", "some comment")
			Expect(watch).To(gbytes.Say(fmt.Sprintf("pinned '%s' with version {\"version\":\"%s\"}\n", inPipeline("some-resource"), someResourceV1)))
			Expect(watch).To(gbytes.Say("pin comment 'some comment' is saved"))

			watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
			Expect(watch).To(gbytes.Say("fetching.*" + someResourceV1))
			Expect(watch).To(gbytes.Say("succeeded"))
		})
	})
})
