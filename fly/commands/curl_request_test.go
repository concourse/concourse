package commands

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
)

var _ = Describe("CurlRequest", func() {

	It("creates an http request", func() {
		requester := CurlRequest{
			Host:   "concourse.foo.com/",
			Path:   "/some/path",
			Method: "PUT",
			Headers: []string{
				"foo: bar:baz",
				"Accept-Encoding: application/json",
			},
			Body: "body-text",
		}

		r, err := requester.CreateHttpRequest()

		Expect(err).To(BeNil())
		Expect(r.URL.Path).To(Equal("concourse.foo.com/some/path"))
		Expect(r.Method).To(Equal("PUT"))
		Expect(r.Header.Get("foo")).To(Equal("bar:baz"))
		Expect(r.Header.Get("Accept-Encoding")).To(Equal("application/json"))

		body, err := ioutil.ReadAll(r.Body)
		Expect(string(body)).To(Equal("body-text"))
	})

	It("errors with invalid header", func() {
		requester := CurlRequest{
			Host:   "concourse.foo.com/",
			Path:   "/some/path",
			Method: "PUT",
			Headers: []string{
				"foo",
			},
		}

		_, err := requester.CreateHttpRequest()
		Expect(err).To(Equal(invalidHeaderError(requester.Headers[0])))
	})

	It("reads body from file", func() {
		f, _ := ioutil.TempFile("", "tempy-temp")
		defer os.Remove(f.Name())

		expectedContent := "Waka waka hazznah"
		f.Write([]byte(expectedContent))

		requester := CurlRequest{
			Host:   "concourse.foo.com/",
			Path:   "/some/path",
			Method: "PUT",
			Headers: []string{
				"foo: bar:baz",
				"Accept-Encoding: application/json",
			},
			Body: fmt.Sprintf("@%s", f.Name()),
		}

		r, err := requester.CreateHttpRequest()

		Expect(err).To(BeNil())
		Expect(r.URL.Path).To(Equal("concourse.foo.com/some/path"))
		Expect(r.Method).To(Equal("PUT"))
		Expect(r.Header.Get("foo")).To(Equal("bar:baz"))
		Expect(r.Header.Get("Accept-Encoding")).To(Equal("application/json"))

		body, err := ioutil.ReadAll(r.Body)
		Expect(string(body)).To(Equal(expectedContent))
	})

	It("returns error if filepath in body doesnt exist", func() {
		badPath := "/tmp/this/will/most/likely/never/exist/i/hope"

		requester := CurlRequest{
			Host:   "concourse.foo.com/",
			Path:   "/some/path",
			Method: "PUT",
			Headers: []string{
				"foo: bar:baz",
				"Accept-Encoding: application/json",
			},
			Body: fmt.Sprintf("@%s", badPath),
		}

		_, err := requester.CreateHttpRequest()

		Expect(err).To(HaveOccurred())
		Expect((err.(*os.PathError)).Path).To(Equal(badPath))
	})

})
