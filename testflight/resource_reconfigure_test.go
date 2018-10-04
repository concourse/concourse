package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Reconfiguring a resource", func() {
	It("picks up the new configuration immediately", func() {
		guid1, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		guid2, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		setAndUnpausePipeline(
			"fixtures/reconfiguring.yml",
			"-v", "force_version="+guid1.String(),
		)

		watch := fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		Expect(watch).To(gbytes.Say(guid1.String()))

		setAndUnpausePipeline(
			"fixtures/reconfiguring.yml",
			"-v", "force_version="+guid2.String(),
		)

		watch = fly("trigger-job", "-j", inPipeline("some-passing-job"), "-w")
		Expect(watch).To(gbytes.Say(guid2.String()))
	})
})
