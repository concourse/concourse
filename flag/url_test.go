package flag_test

import (
	"github.com/concourse/concourse/flag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("URLFlag", func() {
	It("strips slashes from the end of urls", func() {
		flag := flag.URL{}

		err := flag.UnmarshalFlag("http://example.com/")
		Expect(err).ToNot(HaveOccurred())

		Expect(flag.String()).To(Equal("http://example.com"))
	})

	It("doesn't strip anything from urls with no slashes", func() {
		flag := flag.URL{}

		err := flag.UnmarshalFlag("https://example.com")
		Expect(err).ToNot(HaveOccurred())

		Expect(flag.String()).To(Equal("https://example.com"))
	})

	It("returns an error when a scheme is not specified", func() {
		flag := flag.URL{}

		err := flag.UnmarshalFlag("example.com/")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("missing scheme in 'example.com'"))
	})

	It("returns an error for localhost without scheme", func() {
		flag := flag.URL{}

		err := flag.UnmarshalFlag("localhost:8080")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("missing scheme in 'localhost:8080'"))
	})

	It("unmarshalls localhost with scheme correctly", func() {
		flag := flag.URL{}

		err := flag.UnmarshalFlag("http://localhost:8080")
		Expect(err).ToNot(HaveOccurred())

		Expect(flag.String()).To(Equal("http://localhost:8080"))
	})
})
