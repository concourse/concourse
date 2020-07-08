package atc_test

import (
	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ValidateIdentifier", func() {
	Context("when an identifier starts with valid letter", func() {
		It("runs without error", func() {
			err := atc.ValidateIdentifier("something")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when an identifier starts with a number", func() {
		It("returns an error", func() {
			err := atc.ValidateIdentifier("1something")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when an identifier starts with an hyphen", func() {
		It("returns an error", func() {
			err := atc.ValidateIdentifier("-something")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when an identifier starts with an underscore", func() {
		It("returns an error", func() {
			err := atc.ValidateIdentifier("_something")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when an identifier starts with an period", func() {
		It("returns an error", func() {
			err := atc.ValidateIdentifier(".something")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when an identifier contains multilingual characters", func() {
		It("runs without error", func() {
			err := atc.ValidateIdentifier("ひらがな")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
