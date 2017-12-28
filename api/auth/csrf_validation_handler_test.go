package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/api/auth"
	"github.com/concourse/atc/api/auth/authfakes"

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
		fakeUserContextReader *authfakes.FakeUserContextReader
		isCSRFRequired        bool
		logger                *lagertest.TestLogger
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		delegateHandlerCalled = true
	})

	csrfRequiredWrapHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = request.WithContext(context.WithValue(r.Context(), "logger", logger))
		if isCSRFRequired {
			r = request.WithContext(context.WithValue(r.Context(), auth.CSRFRequiredKey, true))
		}
		csrfValidationHandler.ServeHTTP(w, r)
	})

	BeforeEach(func() {
		delegateHandlerCalled = false
		isCSRFRequired = false
		logger = lagertest.NewTestLogger("csrf-validation-test")
		fakeUserContextReader = new(authfakes.FakeUserContextReader)
		csrfValidationHandler = auth.CSRFValidationHandler(
			simpleHandler,
			auth.UnauthorizedRejector{},
			fakeUserContextReader,
		)

		server = httptest.NewServer(csrfRequiredWrapHandler)

		var err error
		request, err = http.NewRequest("POST", server.URL, bytes.NewBufferString("hello"))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		var err error
		response, err = http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
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
					fakeUserContextReader.GetCSRFTokenReturns("", false)
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
					fakeUserContextReader.GetCSRFTokenReturns("some-other-csrf", true)
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
					fakeUserContextReader.GetCSRFTokenReturns("some-csrf-token", true)
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
