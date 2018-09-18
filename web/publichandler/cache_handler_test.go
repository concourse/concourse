package publichandler_test

import (
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/web/publichandler"
)

var _ = Describe("CacheNearlyForever", func() {
	It("adds a cache control header to the wrapped handler", func() {
		insideHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "The wrapped handler was called!")
		})

		wrappedHandler := publichandler.CacheNearlyForever(insideHandler)
		recorder := httptest.NewRecorder()
		request, err := http.NewRequest("GET", "/", nil)
		Expect(err).ToNot(HaveOccurred())

		wrappedHandler.ServeHTTP(recorder, request) // request is never used

		Expect(recorder.Body.String()).To(Equal("The wrapped handler was called!"))
		Expect(recorder.Header().Get("Cache-Control")).To(Equal("max-age=31536000, private"))
		Expect(recorder.Header().Get("Content-Encoding")).ToNot(Equal("gzip"))
	})

	Context("when accept encoding uses gzip", func() {
		It("returns a gzipped asset", func() {

			insideHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, strings.Repeat("abc123", 1000))
			})

			wrappedHandler := publichandler.CacheNearlyForever(insideHandler)
			recorder := httptest.NewRecorder()
			request, err := http.NewRequest("GET", "/", nil)
			Expect(err).ToNot(HaveOccurred())
			request.Header["Accept-Encoding"] = []string{"gzip, deflate, br"}

			wrappedHandler.ServeHTTP(recorder, request) // request is never used

			reader, err := gzip.NewReader(recorder.Body)
			Expect(err).ToNot(HaveOccurred())
			body, err := ioutil.ReadAll(reader)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(body)).To(Equal(strings.Repeat("abc123", 1000)))
			Expect(recorder.Header().Get("Content-Encoding")).To(Equal("gzip"))
		})
	})
})
