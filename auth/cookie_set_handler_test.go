package auth_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

var _ = Describe("CookieSetHandler", func() {
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "auth: %s", r.Header.Get("Authorization"))
	})

	var server *httptest.Server
	var client *http.Client
	username := "username"
	password := "password"

	BeforeEach(func() {
		authHandler := auth.CookieSetHandler{
			Handler: simpleHandler,
		}

		server = httptest.NewServer(authHandler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
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
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		itSetsAuthCookie := func() {
			It("sets a ATC-Authorization cookie with the auth as the value", func() {
				cookies := response.Cookies()
				Ω(cookies).Should(HaveLen(1))

				Ω(cookies[0].Name).Should(Equal("ATC-Authorization"))
				Ω(cookies[0].Value).Should(Equal(header(username, password)))
				Ω(cookies[0].Path).Should(Equal("/"))
				Ω(cookies[0].Expires.Unix()).Should(BeNumerically("~", time.Now().Unix()+60, 1))
			})
		}

		Context("with standard basic auth", func() {
			BeforeEach(func() {
				request.SetBasicAuth(username, password)
			})

			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("proxies to the handler", func() {
				responseBody, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(responseBody)).Should(Equal("auth: " + header(username, password)))
			})

			itSetsAuthCookie()
		})

		Context("with the ATC-Authorization cookie", func() {
			BeforeEach(func() {
				request.AddCookie(&http.Cookie{
					Name:  auth.CookieName,
					Value: header(username, password),
				})
			})

			It("returns 200", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("proxies to the handler with the Authorization header set", func() {
				responseBody, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(responseBody)).Should(Equal("auth: " + header(username, password)))
			})

			itSetsAuthCookie()
		})

		Context("with no credentials", func() {
			It("does not set ATC-Authorization", func() {
				Ω(response.Cookies()).Should(HaveLen(0))
			})
		})
	})
})
