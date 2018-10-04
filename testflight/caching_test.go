package testflight_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Resource caching", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/caching.yml")
	})

	It("does not fetch if there is nothing new", func() {
		someResourceV1 := newMockVersion("some-resource", "some1")
		cachedResourceV1 := newMockVersion("cached-resource", "cached1")

		By("initially fetching twice")
		watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		Expect(watch).To(gbytes.Say("fetching.*" + someResourceV1))
		Expect(watch).To(gbytes.Say("fetching.*" + cachedResourceV1))
		Expect(watch).To(gbytes.Say("succeeded"))

		By("coming up with a new version for one resource")
		someResourceV2 := newMockVersion("some-resource", "some2")

		By("hitting the cache for the original version and fetching the new one")
		watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		Expect(watch).To(gbytes.Say("fetching.*" + someResourceV2))
		Expect(watch).NotTo(gbytes.Say("fetching"))
		Expect(watch).To(gbytes.Say("succeeded"))
		Expect(watch).To(gexec.Exit(0))
	})
})
