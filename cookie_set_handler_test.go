package auth_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

var _ = Describe("CookieSetHandler", func() {
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			fmt.Fprintf(w, "auth: %s", auth)
		}
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

		It("does not set ATC-Authorization", func() {
			Expect(response.Cookies()).To(HaveLen(0))
		})

		It("proxies to the handler without setting the Authorization header", func() {
			responseBody, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(responseBody)).To(Equal(""))
		})

		Context("with the ATC-Authorization cookie", func() {
			BeforeEach(func() {
				request.AddCookie(&http.Cookie{
					Name:  auth.CookieName,
					Value: header("username", "password"),
				})
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("proxies to the handler with the Authorization header set", func() {
				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(ContainSubstring("auth: "))
			})

			It("sets the Authorization header with the value from the cookie", func() {
				responseBody, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(responseBody)).To(ContainSubstring(header("username", "password")))
			})
		})
	})
})
