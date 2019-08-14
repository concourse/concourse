package testflight_test

import (
	uuid "github.com/nu7hatch/gouuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Checking a resource", func() {
	var hash string

	BeforeEach(func() {
		u, err := uuid.NewV4()
		Expect(err).ToNot(HaveOccurred())

		hash = u.String()
	})

	Context("when the resource has a base resource type", func() {
		BeforeEach(func() {
			setAndUnpausePipeline("fixtures/resource-with-params.yml", "-v", "unique_version="+hash)
		})

		It("can check a resource recursively", func() {
			watch := fly("check-resource", "-r", inPipeline("some-git-resource"))
			Expect(watch).To(gbytes.Say("some-git-resource.*succeeded"))
		})
	})
})
