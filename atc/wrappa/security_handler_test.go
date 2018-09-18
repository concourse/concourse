package wrappa_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/wrappa"

	"github.com/concourse/atc/wrappa/wrappafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SecurityHandler", func() {
	var (
		request *http.Request
		rw      *httptest.ResponseRecorder

		fakeHandler *wrappafakes.FakeHandler

		securityHandler wrappa.SecurityHandler
	)

	BeforeEach(func() {
		rw = httptest.NewRecorder()
		request = httptest.NewRequest("GET", "/some/path", nil)

		fakeHandler = new(wrappafakes.FakeHandler)

		securityHandler = wrappa.SecurityHandler{
			Handler: fakeHandler,
		}
	})

	JustBeforeEach(func() {
		securityHandler.ServeHTTP(rw, request)
	})

	It("sets the correct security headers", func() {
		Expect(rw.Header().Get("X-XSS-Protection")).To(Equal("1; mode=block"))
		Expect(rw.Header().Get("X-Content-Type-Options")).To(Equal("nosniff"))
		Expect(rw.Header().Get("X-Download-Options")).To(Equal("noopen"))
	})

	Context("when the X-Frame-Options is empty", func() {
		It("does not set the X-Frame-Options", func() {
			Expect(rw.HeaderMap).NotTo(HaveKey("X-Frame-Options"))
		})
	})

	Context("when the X-Frame-Options is non-empty", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				XFrameOptions: "some-x-frame-options",
				Handler:       fakeHandler,
			}
		})
		It("sets the X-Frame-Options to whatever it was configured with", func() {
			Expect(rw.Header().Get("X-Frame-Options")).To(Equal("some-x-frame-options"))
		})
	})
})
