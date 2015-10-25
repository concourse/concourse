package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Auth API", func() {
	Describe("GET /api/v1/auth/token", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", server.URL+"/api/v1/auth/token", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			for _, b := range []string{"bearer", "BEARER", "Bearer"} {
				bearer := b

				Context("when the request's authorization is already a "+bearer+" token", func() {
					BeforeEach(func() {
						request.Header.Add("Authorization", bearer+" grylls")
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the existing token without generating a new one", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"type":"` + bearer + `","value":"grylls"}`))

						Expect(fakeTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
					})
				})
			}

			Context("when the request's authorization is some other form", func() {
				BeforeEach(func() {
					request.Header.Add("Authorization", "Basic grylls")
				})

				Context("when generating the token succeeds", func() {
					BeforeEach(func() {
						fakeTokenGenerator.GenerateTokenReturns("some type", "some value", nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns a token valid for 1 day", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"type":"some type","value":"some value"}`))

						expiration := fakeTokenGenerator.GenerateTokenArgsForCall(0)
						Expect(expiration).To(BeTemporally("~", time.Now().Add(24*time.Hour), time.Minute))
					})
				})

				Context("when generating the token fails", func() {
					BeforeEach(func() {
						fakeTokenGenerator.GenerateTokenReturns("", "", errors.New("nope"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not generate a token", func() {
				Expect(fakeTokenGenerator.GenerateTokenCallCount()).To(Equal(0))
			})
		})
	})

	Describe("GET /api/v1/auth/methods", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("GET", server.URL+"/api/v1/auth/methods", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("returns the configured providers", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`[
				{
					"type": "oauth",
					"display_name": "OAuth Provider 1",
					"auth_url": "https://example.com/auth/oauth-provider-1"
				},
				{
					"type": "oauth",
					"display_name": "OAuth Provider 2",
					"auth_url": "https://example.com/auth/oauth-provider-2"
				},
				{
					"type": "basic",
					"display_name": "Basic",
					"auth_url": "https://example.com/login/basic"
				}
			]`))
		})
	})
})
