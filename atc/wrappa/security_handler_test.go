package wrappa_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/atc/wrappa"

	"github.com/concourse/concourse/atc/wrappa/wrappafakes"
	. "github.com/onsi/ginkgo/v2"
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
		Expect(rw.Header().Get("X-Content-Type-Options")).To(Equal("nosniff"))
		Expect(rw.Header().Get("X-Download-Options")).To(Equal("noopen"))
		Expect(rw.Header().Get("Cache-Control")).To(Equal("no-store, private"))
	})

	Context("when AdditionalHTTPHeaders is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				AdditionalHTTPHeaders: map[string]string{
					"X-Custom-Header":         "some-custom-value",
					"X-Another-Custom-Header": "another-custom-value",
				},
				Handler: fakeHandler,
			}
		})
		It("sets each header to the configured value", func() {
			Expect(rw.Header().Get("X-Custom-Header")).To(Equal("some-custom-value"))
			Expect(rw.Header().Get("X-Another-Custom-Header")).To(Equal("another-custom-value"))
		})
	})

	Context("when AdditionalHTTPHeaders is empty", func() {
		It("does not set any additional headers", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("X-Custom-Header"))
		})
	})

	Context("when the X-Frame-Options is empty", func() {
		It("does not set the X-Frame-Options", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("X-Frame-Options"))
		})
	})

	Context("when the X-Frame-Options is set", func() {
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

	Context("when Content-Security-Policy is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				ContentSecurityPolicy: "some-policy 'value'",
				Handler:               fakeHandler,
			}
		})
		It("sets the Content-Security-Policy to whatever it was configured with", func() {
			Expect(rw.Header().Get("Content-Security-Policy")).To(Equal("some-policy 'value'"))
		})
	})

	Context("when Content-Security-Policy is empty", func() {
		It("does not set Content-Security-Policy header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Content-Security-Policy"))
		})
	})

	Context("when Strict-Transport-Security is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				StrictTransportSecurity: "some-policy 'value'",
				Handler:                 fakeHandler,
			}
		})
		It("sets the Strict-Transport-Security to whatever it was configured with", func() {
			Expect(rw.Header().Get("Strict-Transport-Security")).To(Equal("some-policy 'value'"))
		})
	})

	Context("when Strict-Transport-Security is empty", func() {
		It("does not set Strict-Transport-Security header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Strict-Transport-Security"))
		})
	})
})
