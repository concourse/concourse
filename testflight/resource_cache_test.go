package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Resource caching", func() {
	It("takes params into account when determining a cache hit", func() {
		guid, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/resource-with-params.yml",
			"-v", "unique_version="+guid.String(),
		)

		watch := fly("trigger-job", "-j", inPipeline("without-params"), "-w")
		Expect(watch).To(gbytes.Say("fetching.*" + guid.String()))

		watch = fly("trigger-job", "-j", inPipeline("without-params"), "-w")
		Expect(watch).ToNot(gbytes.Say("fetching"))

		watch = fly("trigger-job", "-j", inPipeline("with-params"), "-w")
		Expect(watch).To(gbytes.Say("fetching.*" + guid.String()))
	})
})
