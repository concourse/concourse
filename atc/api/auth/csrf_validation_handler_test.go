package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/skymarshal/token/tokenfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CsrfValidationHandler", func() {
	var (
		server                *httptest.Server
		csrfValidationHandler http.Handler
		request               *http.Request
		response              *http.Response
		delegateHandlerCalled bool
		fakeMiddleware        *tokenfakes.FakeMiddleware
		isCSRFRequired        bool
		logger                *lagertest.TestLogger
		isLoggerSet           bool
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delegateHandlerCalled = true
	})

	csrfRequiredWrapHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isLoggerSet {
			r = request.WithContext(context.WithValue(r.Context(), "logger", logger))
		}
		if isCSRFRequired {
			r = request.WithContext(context.WithValue(r.Context(), auth.CSRFRequiredKey, true))
		}
		csrfValidationHandler.ServeHTTP(w, r)
	})

	BeforeEach(func() {
		isLoggerSet = true
		fakeMiddleware = new(tokenfakes.FakeMiddleware)
		delegateHandlerCalled = false
		isCSRFRequired = false
		logger = lagertest.NewTestLogger("csrf-validation-test")

		csrfValidationHandler = auth.CSRFValidationHandler(
			simpleHandler,
			fakeMiddleware,
		)

		server = httptest.NewServer(csrfRequiredWrapHandler)

		var err error
		request, err = http.NewRequest("POST", server.URL, bytes.NewBufferString("hello"))
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		response, err = http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when request does not require CSRF validation", func() {
		Context("when CSRF token is not provided", func() {
			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("calls delegate handler", func() {
				Expect(delegateHandlerCalled).To(BeTrue())
			})
		})
	})

	Context("when request requires CSRF validation", func() {
		BeforeEach(func() {
			isCSRFRequired = true
		})

		Context("when GET request", func() {
			BeforeEach(func() {
				var err error
				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).NotTo(HaveOccurred())

				request.Header.Set(auth.CSRFHeaderName, "some-token")
				fakeMiddleware.GetCSRFTokenReturns("some-token")
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("calls delegate handler", func() {
				Expect(delegateHandlerCalled).To(BeTrue())
			})
		})

		Context("when CSRF token is not provided", func() {
			It("returns 401 Bad Request", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not call delegate handler", func() {
				Expect(delegateHandlerCalled).To(BeFalse())
			})
		})

		Context("when CSRF token is provided", func() {
			BeforeEach(func() {
				request.Header.Set(auth.CSRFHeaderName, "some-csrf-token")
			})

			Context("when auth token does not contain CSRF", func() {
				BeforeEach(func() {
					fakeMiddleware.GetCSRFTokenReturns("")
				})

				It("returns 401 Bad Request", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not call delegate handler", func() {
					Expect(delegateHandlerCalled).To(BeFalse())
				})
			})

			Context("when auth token contains non-matching CSRF", func() {
				BeforeEach(func() {
					fakeMiddleware.GetCSRFTokenReturns("some-other-csrf")
				})

				It("returns 401 Not Authorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not call delegate handler", func() {
					Expect(delegateHandlerCalled).To(BeFalse())
				})
			})

			Context("when auth token contains matching CSRF", func() {
				BeforeEach(func() {
					fakeMiddleware.GetCSRFTokenReturns("some-csrf-token")
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("calls delegate handler", func() {
					Expect(delegateHandlerCalled).To(BeTrue())
				})
			})
		})
	})
})
