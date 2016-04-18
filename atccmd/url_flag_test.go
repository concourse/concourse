package atccmd_test

import (
	"github.com/concourse/atc/atccmd"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("URLFlag", func() {
	It("strips slashes from the end of urls", func() {
		flag := atccmd.URLFlag{}

		err := flag.UnmarshalFlag("http://example.com/")
		Expect(err).ToNot(HaveOccurred())

		Expect(flag.String()).To(Equal("http://example.com"))
	})

	It("doesn't strip anything from urls with no slashes", func() {
		flag := atccmd.URLFlag{}

		err := flag.UnmarshalFlag("https://example.com")
		Expect(err).ToNot(HaveOccurred())

		Expect(flag.String()).To(Equal("https://example.com"))
	})

	It("returns an error when a scheme is not specified", func() {
		flag := atccmd.URLFlag{}

		err := flag.UnmarshalFlag("example.com/")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("missing scheme in 'example.com'"))
	})
})
