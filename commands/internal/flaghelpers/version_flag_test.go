package flaghelpers_test

import (
	. "github.com/concourse/fly/commands/internal/flaghelpers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionFlag", func() {
	It("parses key and value", func() {
		versionFlag := &VersionFlag{}
		err := versionFlag.UnmarshalFlag("ref:abcdef")
		Expect(err).NotTo(HaveOccurred())

		Expect(versionFlag).To(Equal(&VersionFlag{Key: "ref", Value: "abcdef"}))
	})

	Context("when there is only a key specified", func() {
		It("displays an error message", func() {
			versionFlag := &VersionFlag{}

			err := versionFlag.UnmarshalFlag("ref")
			Expect(err).To(MatchError("invalid version pair 'ref' (must be key:value)"))
		})
	})
})
