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

	Context("when Referrer-Policy is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				ReferrerPolicy: "strict-origin-when-cross-origin",
				Handler:        fakeHandler,
			}
		})
		It("sets the Referrer-Policy to whatever it was configured with", func() {
			Expect(rw.Header().Get("Referrer-Policy")).To(Equal("strict-origin-when-cross-origin"))
		})
	})

	Context("when Referrer-Policy is empty", func() {
		It("does not set Referrer-Policy header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Referrer-Policy"))
		})
	})

	Context("when Cross-Origin-Opener-Policy is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				CrossOriginOpenerPolicy: "same-origin",
				Handler:                 fakeHandler,
			}
		})
		It("sets the Cross-Origin-Opener-Policy to whatever it was configured with", func() {
			Expect(rw.Header().Get("Cross-Origin-Opener-Policy")).To(Equal("same-origin"))
		})
	})

	Context("when Cross-Origin-Opener-Policy is empty", func() {
		It("does not set Cross-Origin-Opener-Policy header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Cross-Origin-Opener-Policy"))
		})
	})

	Context("when Cross-Origin-Resource-Policy is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				CrossOriginResourcePolicy: "same-site",
				Handler:                   fakeHandler,
			}
		})
		It("sets the Cross-Origin-Resource-Policy to whatever it was configured with", func() {
			Expect(rw.Header().Get("Cross-Origin-Resource-Policy")).To(Equal("same-site"))
		})
	})

	Context("when Cross-Origin-Resource-Policy is empty", func() {
		It("does not set Cross-Origin-Resource-Policy header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Cross-Origin-Resource-Policy"))
		})
	})

	Context("when Cross-Origin-Embedder-Policy is set", func() {
		BeforeEach(func() {
			securityHandler = wrappa.SecurityHandler{
				CrossOriginEmbedderPolicy: "require-corp",
				Handler:                   fakeHandler,
			}
		})
		It("sets the Cross-Origin-Embedder-Policy to whatever it was configured with", func() {
			Expect(rw.Header().Get("Cross-Origin-Embedder-Policy")).To(Equal("require-corp"))
		})
	})

	Context("when Cross-Origin-Embedder-Policy is empty", func() {
		It("does not set Cross-Origin-Embedder-Policy header", func() {
			Expect(rw.Result().Header).NotTo(HaveKey("Cross-Origin-Embedder-Policy"))
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
