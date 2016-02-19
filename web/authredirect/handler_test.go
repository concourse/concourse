package authredirect_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/concourse/atc/web/authredirect"
	"github.com/concourse/atc/web/authredirect/fakes"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var fakeErrHandler *fakes.FakeErrHandler

	var handler http.Handler
	var server *httptest.Server

	var transport *http.Transport
	var request *http.Request
	var response *http.Response
	var requestErr error

	BeforeEach(func() {
		fakeErrHandler = new(fakes.FakeErrHandler)
		handler = authredirect.Tracker{authredirect.Handler{fakeErrHandler}}

		server = httptest.NewServer(handler)

		transport = &http.Transport{}

		var err error
		request, err = http.NewRequest("GET", server.URL+"/some-path", nil)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		server.Close()
	})

	JustBeforeEach(func() {
		response, requestErr = transport.RoundTrip(request)
	})

	Context("when the ErrHandler returns nil", func() {
		BeforeEach(func() {
			fakeErrHandler.ServeHTTPReturns(nil)
		})

		It("does nothing with the response writer", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Context("when the ErrHandler returns concourse.ErrUnauthorized", func() {
		BeforeEach(func() {
			fakeErrHandler.ServeHTTPReturns(concourse.ErrUnauthorized)
		})

		Context("when the request was a GET", func() {
			It("redirects to /login?redirect=<request uri>", func() {
				Expect(response.StatusCode).To(Equal(http.StatusFound))
				Expect(response.Header.Get("Location")).To(Equal("/login?" + url.Values{
					"redirect": {"/some-path"},
				}.Encode()))
			})
		})

		for _, method := range []string{"POST", "PUT", "DELETE"} {
			method := method

			Context("when the request was a "+method, func() {
				BeforeEach(func() {
					var err error
					request, err = http.NewRequest(method, server.URL+"/some-path", nil)
					Expect(err).ToNot(HaveOccurred())
				})

				Context("with a Referer header", func() {
					BeforeEach(func() {
						request.Header.Set("Referer", "http://referer.com")
					})

					It("redirects to /login?redirect=<referer uri>", func() {
						Expect(response.StatusCode).To(Equal(http.StatusFound))
						Expect(response.Header.Get("Location")).To(Equal("/login?" + url.Values{
							"redirect": {"http://referer.com"},
						}.Encode()))
					})
				})

				Context("without a Referer header", func() {
					It("redirects to /login with no redirect", func() {
						Expect(response.StatusCode).To(Equal(http.StatusFound))
						Expect(response.Header.Get("Location")).To(Equal("/login"))
					})
				})
			})
		}
	})

	Context("when the ErrHandler returns some other error", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			fakeErrHandler.ServeHTTPReturns(disaster)
		})

		It("returns Internal Server Error", func() {
			Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
		})
	})
})
