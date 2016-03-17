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

	It("can detect development versions", func() {
		isDev := IsDev("1.2.3")
		Expect(isDev).To(BeFalse())

		isDev = IsDev("0.0.0-dev")
		Expect(isDev).To(BeTrue())
	})
})
