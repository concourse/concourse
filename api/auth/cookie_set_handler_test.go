package auth_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/api/auth"
)

var _ = Describe("CookieSetHandler", func() {
	var (
		givenRequest *http.Request
	)

	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		givenRequest = r
	})

	var server *httptest.Server

	BeforeEach(func() {
		server = httptest.NewServer(auth.CookieSetHandler{
			Handler: simpleHandler,
		})
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("handling a request", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not set auth cookie", func() {
			Expect(response.Cookies()).To(HaveLen(0))
		})

		It("proxies to the handler without setting the Authorization header", func() {
			Expect(givenRequest.Header.Get("Authorization")).To(BeEmpty())
		})

		It("does not set CSRF required context in request", func() {
			csrfRequiredContext := givenRequest.Context().Value(auth.CSRFRequiredKey)
			Expect(csrfRequiredContext).To(BeNil())
		})

		Context("with the auth cookie", func() {
			BeforeEach(func() {
				request.AddCookie(&http.Cookie{
					Name:  auth.AuthCookieName,
					Value: "username:password",
				})
			})

			It("sets the Authorization header with the value from the cookie", func() {
				Expect(givenRequest.Header.Get("Authorization")).To(Equal("username:password"))
			})

			It("sets CSRF required context in request", func() {
				csrfRequiredContext := givenRequest.Context().Value(auth.CSRFRequiredKey)
				Expect(csrfRequiredContext).NotTo(BeNil())

				boolCsrf := csrfRequiredContext.(bool)
				Expect(boolCsrf).To(BeFalse())
			})

			Context("and the request also has an Authorization header", func() {
				BeforeEach(func() {
					request.Header.Set("Authorization", "foobar")
				})

				It("does not override the Authorization header", func() {
					Expect(givenRequest.Header.Get("Authorization")).To(Equal("foobar"))
				})
			})
		})

	})
	Describe("CSRF Required", func() {
		var request *http.Request
		var err error
		Context("when CSRF context is set", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).To(BeNil())

				ctx := context.WithValue(request.Context(), auth.CSRFRequiredKey, true)
				request = request.WithContext(ctx)

			})
			It("fetches the bool value", func() {
				Expect(auth.IsCSRFRequired(request)).To(BeTrue())
			})
		})

		Context("when CSRF context is not set", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
				Expect(err).To(BeNil())
			})
			It("fetches the bool value", func() {
				Expect(auth.IsCSRFRequired(request)).To(BeFalse())
			})

		})
	})
})
