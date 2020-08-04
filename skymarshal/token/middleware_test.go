package token_test

import (
	"time"

	"net/http"
	"net/http/httptest"

	"github.com/concourse/concourse/skymarshal/token"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Token Middleware", func() {

	var (
		err        error
		expiry     time.Time
		r          *http.Request
		w          *httptest.ResponseRecorder
		middleware token.Middleware
	)

	BeforeEach(func() {
		expiry = time.Now().Add(time.Minute)

		r, err = http.NewRequest("GET", "http://example.come", nil)
		Expect(err).NotTo(HaveOccurred())

		w = httptest.NewRecorder()

		middleware = token.NewMiddleware(false)
	})

	Describe("Auth Tokens", func() {
		Describe("GetAuthToken", func() {
			var result string

			BeforeEach(func() {
				r.AddCookie(&http.Cookie{Name: "skymarshal_auth", Value: "blah"})
			})

			JustBeforeEach(func() {
				result = middleware.GetAuthToken(r)
			})

			It("gets the token from the request", func() {
				Expect(result).To(Equal("blah"))
			})
		})

		Describe("SetAuthToken", func() {
			JustBeforeEach(func() {
				err = middleware.SetAuthToken(w, "blah", expiry)
			})

			It("writes the token to a cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))

				Expect(cookies[0].Name).To(Equal("skymarshal_auth"))
				Expect(cookies[0].Expires.Unix()).To(Equal(expiry.Unix()))
				Expect(cookies[0].Value).To(Equal("blah"))
			})
		})

		Describe("UnsetAuthToken", func() {
			JustBeforeEach(func() {
				middleware.UnsetAuthToken(w)
			})

			It("clears the token from the cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))
				Expect(cookies[0].Name).To(Equal("skymarshal_auth"))
				Expect(cookies[0].Value).To(Equal(""))
			})
		})
	})

	Describe("CSRF Tokens", func() {

		Describe("GetCSRFToken", func() {
			var result string

			BeforeEach(func() {
				r.AddCookie(&http.Cookie{Name: "skymarshal_csrf", Value: "blah"})
			})

			JustBeforeEach(func() {
				result = middleware.GetCSRFToken(r)
			})

			It("gets the token from the request", func() {
				Expect(result).To(Equal("blah"))
			})
		})

		Describe("SetCSRFToken", func() {
			JustBeforeEach(func() {
				err = middleware.SetCSRFToken(w, "blah", expiry)
			})

			It("writes the token to a cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))
				Expect(cookies[0].Name).To(Equal("skymarshal_csrf"))
				Expect(cookies[0].Expires.Unix()).To(Equal(expiry.Unix()))
				Expect(cookies[0].Value).To(Equal("blah"))
			})
		})

		Describe("UnsetCSRFToken", func() {
			JustBeforeEach(func() {
				middleware.UnsetCSRFToken(w)
			})

			It("clears the token from the cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))
				Expect(cookies[0].Name).To(Equal("skymarshal_csrf"))
				Expect(cookies[0].Value).To(Equal(""))
			})
		})
	})

	Describe("State Tokens", func() {

		Describe("GetStateToken", func() {
			var result string

			BeforeEach(func() {
				r.AddCookie(&http.Cookie{Name: "skymarshal_state", Value: "blah"})
			})

			JustBeforeEach(func() {
				result = middleware.GetStateToken(r)
			})

			It("gets the token from the request", func() {
				Expect(result).To(Equal("blah"))
			})
		})

		Describe("SetStateToken", func() {
			JustBeforeEach(func() {
				err = middleware.SetStateToken(w, "blah", expiry)
			})

			It("writes the token to a cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))
				Expect(cookies[0].Name).To(Equal("skymarshal_state"))
				Expect(cookies[0].Expires.Unix()).To(Equal(expiry.Unix()))
				Expect(cookies[0].Value).To(Equal("blah"))
			})
		})

		Describe("UnsetStateToken", func() {
			JustBeforeEach(func() {
				middleware.UnsetStateToken(w)
			})

			It("clears the token from the cookie", func() {
				cookies := w.Result().Cookies()
				Expect(cookies).To(HaveLen(1))
				Expect(cookies[0].Name).To(Equal("skymarshal_state"))
				Expect(cookies[0].Value).To(Equal(""))
			})
		})
	})
})
