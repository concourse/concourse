package flag_test

import (
	"os"
	"path/filepath"

	"github.com/concourse/concourse/v8/flag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CustomHTTPHeaders", func() {
	var (
		f   flag.CustomHTTPHeaders
		tmp string
	)

	BeforeEach(func() {
		f = flag.CustomHTTPHeaders{}
		var err error
		tmp, err = os.MkdirTemp("", "custom-http-headers-test")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmp)
	})

	It("returns an error when the file does not exist", func() {
		err := f.UnmarshalFlag("/nonexistent/headers.yml")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to open"))
	})

	It("returns an error when the file contains invalid YAML", func() {
		path := filepath.Join(tmp, "invalid.yml")
		Expect(os.WriteFile(path, []byte("X-Custom-Header; custom-value\n"), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse"))
	})

	It("returns an error when a header name is invalid", func() {
		path := filepath.Join(tmp, "invalid-name.yml")
		Expect(os.WriteFile(path, []byte("X Custom Header: custom-value\n"), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid header name"))
	})

	It("parses a valid YAML file correctly", func() {
		path := filepath.Join(tmp, "valid.yml")
		Expect(os.WriteFile(path, []byte("X-Custom-Header: custom-value\nX-Another-Header: \"some; value, string\"\n"), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.Headers).To(Equal(map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Another-Header": "some; value, string",
		}))
		Expect(f.Path).To(Equal(path))
	})

	It("parses a valid JSON file correctly", func() {
		path := filepath.Join(tmp, "valid.json")
		Expect(os.WriteFile(path, []byte(`{"X-Custom-Header": "custom-value", "X-Another-Header": "some; value, string"}`), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.Headers).To(Equal(map[string]string{
			"X-Custom-Header":  "custom-value",
			"X-Another-Header": "some; value, string",
		}))
	})

	It("returns an empty map for an empty file", func() {
		path := filepath.Join(tmp, "empty.yml")
		Expect(os.WriteFile(path, []byte(""), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.Headers).To(Equal(map[string]string{}))
	})

	It("allows empty string as a header value", func() {
		path := filepath.Join(tmp, "empty-value.yml")
		Expect(os.WriteFile(path, []byte("X-Content-Type-Options: \"\"\n"), 0644)).To(Succeed())

		err := f.UnmarshalFlag(path)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.Headers).To(Equal(map[string]string{
			"X-Content-Type-Options": "",
		}))
	})
})
