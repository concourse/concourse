package version_test

import (
	. "github.com/concourse/fly/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	It("parses versions", func() {
		major, minor, patch, err := GetSemver("1.2.3")
		Expect(err).ToNot(HaveOccurred())
		Expect(major).To(Equal(1))
		Expect(minor).To(Equal(2))
		Expect(patch).To(Equal(3))
	})

	It("returns an error for non-parsable versions", func() {
		_, _, _, err := GetSemver("")
		Expect(err).To(HaveOccurred())
		_, _, _, err = GetSemver("1.2")
		Expect(err).To(HaveOccurred())
	})

	It("parses pre-release versions", func() {
		major, minor, patch, err := GetSemver("1.2.3-dev.2")
		Expect(err).To(Succeed())
		Expect(major).To(Equal(1))
		Expect(minor).To(Equal(2))
		Expect(patch).To(Equal(3))
	})

	It("parses post-release versions", func() {
		major, minor, patch, err := GetSemver("1.2.3+bonus_feature.1")
		Expect(err).To(Succeed())
		Expect(major).To(Equal(1))
		Expect(minor).To(Equal(2))
		Expect(patch).To(Equal(3))
	})

	It("can detect development versions", func() {
		Expect(IsDev("1.2.3")).To(BeFalse())
		Expect(IsDev("0.0.0-dev")).To(BeTrue())

		Expect(IsDev("0.0.0-devolve")).To(BeFalse())
		Expect(IsDev("0.0.0-not-dev")).To(BeFalse())
		Expect(IsDev("0.0.0+not+dev")).To(BeFalse())

		Expect(IsDev("0.0.0-dev.1")).To(BeTrue())
		Expect(IsDev("0.0.0-abc+dev")).To(BeTrue())
		Expect(IsDev("0.0.0-abc+dev.1")).To(BeTrue())
		Expect(IsDev("0.0.0-dev+dev")).To(BeTrue())
		Expect(IsDev("0.0.0-abc+dev.1")).To(BeTrue())
	})
})
