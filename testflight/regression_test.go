package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Regression tests", func() {
	Describe("issue 7282", func() {
		It("does not error when resources emit long metadata strings", func() {
			setAndUnpausePipeline("fixtures/long-metadata.yml")

			watch := fly("trigger-job", "-j", inPipeline("job"), "-w")
			Expect(watch).To(gexec.Exit(0))
		})
	})
})
