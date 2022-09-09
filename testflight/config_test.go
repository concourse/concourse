package testflight_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Configuring a resource with nested configuration", func() {
	BeforeEach(func() {
		setAndUnpausePipeline("fixtures/nested-config-test.yml")
	})

	It("works", func() {
		watch := fly("trigger-job", "-j", inPipeline("config-test"), "-w")
		Expect(watch).To(gexec.Exit(0))
	})
})
