package publichandler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/web/publichandler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CacheNearlyForever", func() {
	It("adds a cache control header to the wrapped handler", func() {
		insideHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "The wrapped handler was called!")
		})

		wrappedHandler := publichandler.CacheNearlyForever(insideHandler)
		recorder := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(recorder, nil) // request is never used

		Expect(recorder.Body.String()).To(Equal("The wrapped handler was called!"))
		Expect(recorder.Header().Get("Cache-Control")).To(Equal("max-age=31536000, private"))
	})
})
