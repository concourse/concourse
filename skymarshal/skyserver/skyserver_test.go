package skyserver_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

// stateToken mirrors the struct in skyserver.go for test signing
type stateToken struct {
	RedirectURI string `json:"redirect_uri"`
	Entropy     string `json:"entropy"`
	Timestamp   int64  `json:"ts"`
	Signature   string `json:"sig,omitempty"`
}

// signStateToken creates a signed state token for testing
func signStateToken(redirectURI string, ts int64) string {
	st := stateToken{
		RedirectURI: redirectURI,
		Entropy:     "test-entropy",
		Timestamp:   ts,
	}

	payload, _ := json.Marshal(st)

	mac := hmac.New(sha256.New, stateSigningKey)
	mac.Write(payload)
	st.Signature = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	signed, _ := json.Marshal(st)
	return base64.RawURLEncoding.EncodeToString(signed)
}

var _ = Describe("Sky Server API", func() {

	ExpectServerBehaviour := func() {

		Describe("GET /sky/login", func() {
			var (
				err      error
				request  *http.Request
				response *http.Response
			)

			BeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/login", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				skyServer.Client().CheckRedirect = func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				}

				response, err = skyServer.Client().Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			ExpectNewLogin := func() {

				It("redirects the initial request to the oauthConfig.AuthURL", func() {
					redirectURL, err := response.Location()
					Expect(err).NotTo(HaveOccurred())
					Expect(redirectURL.Path).To(Equal("/auth"))

					redirectValues := redirectURL.Query()
					Expect(redirectValues.Get("access_type")).To(Equal("offline"))
					Expect(redirectValues.Get("response_type")).To(Equal("code"))
					Expect(redirectValues.Get("state")).NotTo(BeEmpty())
					Expect(redirectValues.Get("scope")).To(Equal("some-scope"))
				})

				Context("when redirect_uri is provided", func() {
					BeforeEach(func() {
						request.URL.RawQuery = "redirect_uri=/redirect"
					})

					It("stores redirect_uri in the signed state token", func() {
						redirectURL, err := response.Location()
						Expect(err).NotTo(HaveOccurred())

						stateParam := redirectURL.Query().Get("state")
						data, err := base64.RawURLEncoding.DecodeString(stateParam)
						Expect(err).NotTo(HaveOccurred())

						var state map[string]any
						json.Unmarshal(data, &state)
						Expect(state["redirect_uri"]).To(Equal("/redirect"))
						Expect(state["sig"]).NotTo(BeEmpty())
						Expect(state["ts"]).NotTo(BeZero())
					})
				})

				Context("when redirect_uri is NOT provided", func() {
					BeforeEach(func() {
						request.URL.RawQuery = ""
					})

					It("stores / as the default redirect_uri in the signed state token", func() {
						redirectURL, err := response.Location()
						Expect(err).NotTo(HaveOccurred())

						stateParam := redirectURL.Query().Get("state")
						data, err := base64.RawURLEncoding.DecodeString(stateParam)
						Expect(err).NotTo(HaveOccurred())

						var state map[string]any
						json.Unmarshal(data, &state)
						Expect(state["redirect_uri"]).To(Equal("/"))
					})
				})
			}

			Context("without an existing token", func() {
				BeforeEach(func() {
					fakeTokenMiddleware.GetAuthTokenReturns("")
				})
				ExpectNewLogin()
			})

			Context("when the token has no type", func() {
				BeforeEach(func() {
					fakeTokenMiddleware.GetAuthTokenReturns("some-token")
				})
				ExpectNewLogin()
			})

			Context("when the token is not a valid bearer token", func() {
				BeforeEach(func() {
					fakeTokenMiddleware.GetAuthTokenReturns("not-bearer some-token")
				})
				ExpectNewLogin()
			})

			Context("when parsing the expiry errors", func() {
				BeforeEach(func() {
					fakeTokenParser.ParseExpiryReturns(time.Time{}, errors.New("error"))
					fakeTokenMiddleware.GetAuthTokenReturns("bearer some-token")
				})
				ExpectNewLogin()
			})

			Context("when the token is expired", func() {
				BeforeEach(func() {
					fakeTokenParser.ParseExpiryReturns(time.Now().Add(-time.Hour), nil)
					fakeTokenMiddleware.GetAuthTokenReturns("bearer some-token")
				})
				ExpectNewLogin()
			})

			Context("when the token is valid", func() {
				BeforeEach(func() {
					fakeTokenParser.ParseExpiryReturns(time.Now().Add(time.Hour), nil)
					fakeTokenMiddleware.GetAuthTokenReturns("bearer some-token")
				})

				It("updates the auth token", func() {
					Expect(fakeTokenMiddleware.SetAuthTokenCallCount()).To(Equal(1))
					_, tokenArg, _ := fakeTokenMiddleware.SetAuthTokenArgsForCall(0)
					Expect(tokenArg).To(Equal("bearer some-token"))
				})

				It("updates the csrf token", func() {
					Expect(fakeTokenMiddleware.SetCSRFTokenCallCount()).To(Equal(1))
					_, tokenArg, _ := fakeTokenMiddleware.SetCSRFTokenArgsForCall(0)
					Expect(tokenArg).NotTo(BeEmpty())
				})

				It("redirects the request to the provided redirect_uri", func() {
					_, tokenArg, _ := fakeTokenMiddleware.SetCSRFTokenArgsForCall(0)

					redirectURL, err := response.Location()
					Expect(err).NotTo(HaveOccurred())

					atcURL, err := url.Parse(skyServer.URL)
					Expect(err).NotTo(HaveOccurred())
					Expect(redirectURL.Host).To(Equal(atcURL.Host))

					redirectValues := redirectURL.Query()
					Expect(redirectValues.Get("csrf_token")).To(Equal(tokenArg))
				})
			})
		})

		Describe("GET /sky/logout", func() {
			var (
				err      error
				request  *http.Request
				response *http.Response
			)

			BeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/logout", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				response, err = skyServer.Client().Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when there is no auth token", func() {
				BeforeEach(func() {
					fakeTokenMiddleware.GetAuthTokenReturns("")
				})

				It("returns unauthorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not try to delete anything", func() {
					Expect(fakeClaimsCacher.DeleteAccessTokenCallCount()).To(Equal(0))
					Expect(fakeAccessTokenFactory.DeleteAccessTokenCallCount()).To(Equal(0))
				})
			})

			Context("when there is an auth token", func() {
				BeforeEach(func() {
					fakeTokenMiddleware.GetAuthTokenReturns("bearer some-token")
				})

				Context("when deleting from the cache fails", func() {
					BeforeEach(func() {
						fakeClaimsCacher.DeleteAccessTokenReturns(errors.New("cache failed"))
					})

					It("returns an internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					It("does not try to delete from the DB", func() {
						Expect(fakeClaimsCacher.DeleteAccessTokenCallCount()).To(Equal(1))
						Expect(fakeAccessTokenFactory.DeleteAccessTokenCallCount()).To(Equal(0))
					})
				})

				Context("when deleting from the DB fails", func() {
					BeforeEach(func() {
						fakeAccessTokenFactory.DeleteAccessTokenReturns(errors.New("db failed"))
					})

					It("returns an internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})

					It("calls DeleteAccessToken on cache first", func() {
						Expect(fakeClaimsCacher.DeleteAccessTokenCallCount()).To(Equal(1))
						Expect(fakeClaimsCacher.DeleteAccessTokenArgsForCall(0)).To(Equal("some-token"))
						Expect(fakeAccessTokenFactory.DeleteAccessTokenCallCount()).To(Equal(1))
					})
				})

				Context("when everything succeeds", func() {
					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("deletes from cache and DB", func() {
						Expect(fakeClaimsCacher.DeleteAccessTokenCallCount()).To(Equal(1))
						Expect(fakeClaimsCacher.DeleteAccessTokenArgsForCall(0)).To(Equal("some-token"))

						Expect(fakeAccessTokenFactory.DeleteAccessTokenCallCount()).To(Equal(1))
						Expect(fakeAccessTokenFactory.DeleteAccessTokenArgsForCall(0)).To(Equal("some-token"))
					})

					It("unsets the auth and csrf tokens", func() {
						Expect(fakeTokenMiddleware.UnsetAuthTokenCallCount()).To(Equal(1))
						Expect(fakeTokenMiddleware.UnsetCSRFTokenCallCount()).To(Equal(1))
					})
				})
			})
		})

		Describe("GET /sky/callback", func() {
			var (
				err      error
				request  *http.Request
				response *http.Response
				body     []byte
			)

			BeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback", nil)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				response, err = skyServer.Client().Do(request)
				Expect(err).NotTo(HaveOccurred())

				body, err = io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when there's an error param", func() {
				BeforeEach(func() {
					request.URL.RawQuery = "error=some-error"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("shows the error message", func() {
					Expect(string(body)).To(Equal("some-error\n"))
				})
			})

			Context("when the state token is missing", func() {
				BeforeEach(func() {
					request.URL.RawQuery = ""
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("shows invalid state message", func() {
					Expect(string(body)).To(Equal("invalid state token\n"))
				})
			})

			Context("when the state token has invalid signature", func() {
				BeforeEach(func() {
					// Create unsigned state token
					st := stateToken{
						RedirectURI: "/redirect",
						Entropy:     "test",
						Timestamp:   time.Now().Unix(),
						Signature:   "invalid-signature",
					}
					data, _ := json.Marshal(st)
					invalidState := base64.RawURLEncoding.EncodeToString(data)
					request.URL.RawQuery = "state=" + invalidState
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("shows invalid state message", func() {
					Expect(string(body)).To(Equal("invalid state token\n"))
				})
			})

			Context("when the state token is expired", func() {
				BeforeEach(func() {
					// Create state token with old timestamp (2 hours ago)
					expiredState := signStateToken("/redirect", time.Now().Add(-2*time.Hour).Unix())
					request.URL.RawQuery = "state=" + expiredState
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("shows invalid state message", func() {
					Expect(string(body)).To(Equal("invalid state token\n"))
				})
			})

			Context("when the state token is valid", func() {
				var validState string

				BeforeEach(func() {
					validState = signStateToken("/some-redirect", time.Now().Unix())
					request.URL.RawQuery = "state=" + validState
				})

				Context("when there is an authorization code", func() {
					BeforeEach(func() {
						request.URL.RawQuery = "code=some-code&state=" + validState
					})

					Context("when requesting a token fails", func() {
						BeforeEach(func() {
							dexServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/token"),
									ghttp.VerifyHeaderKV("Authorization", "Basic ZGV4LWNsaWVudC1pZDpkZXgtY2xpZW50LXNlY3JldA=="),
									ghttp.VerifyFormKV("grant_type", "authorization_code"),
									ghttp.VerifyFormKV("code", "some-code"),
									ghttp.RespondWith(http.StatusInternalServerError, "some-token-error"),
								),
							)
						})

						It("errors", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})

						It("shows the oauth2 retrieve error response", func() {
							Expect(string(body)).To(Equal("some-token-error\n"))
						})
					})

					Context("when requesting a token from dex fails with oauth error (dex 200 with no access_token returned)", func() {
						BeforeEach(func() {
							dexServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/token"),
									ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]string{
										"token_type": "some-type",
										"id_token":   "some-id-token",
									}),
								),
							)
						})

						It("errors", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("shows oauth error", func() {
							Expect(string(body)).To(Equal("oauth2: server response missing access_token\n"))
						})
					})

					Context("when the server returns a token", func() {

						BeforeEach(func() {
							dexServer.AppendHandlers(
								ghttp.CombineHandlers(
									ghttp.VerifyRequest("POST", "/token"),
									ghttp.VerifyHeaderKV("Authorization", "Basic ZGV4LWNsaWVudC1pZDpkZXgtY2xpZW50LXNlY3JldA=="),
									ghttp.VerifyFormKV("grant_type", "authorization_code"),
									ghttp.VerifyFormKV("code", "some-code"),
									ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]string{
										"token_type":   "some-type",
										"access_token": "some-token",
										"id_token":     "some-id-token",
									}),
								),
							)
						})

						Context("when redirect URI is http://example.com", func() {
							BeforeEach(func() {
								stateToken := signStateToken("http://example.com", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when redirect URI is //example.com", func() {
							BeforeEach(func() {
								stateToken := signStateToken("//example.com", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when redirect URI is http:///example.com/path", func() {
							BeforeEach(func() {
								stateToken := signStateToken("http:///example.com/path", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when redirect URI is https:example.com", func() {
							BeforeEach(func() {
								stateToken := signStateToken("https:example.com", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when redirect URI is example.com", func() {
							BeforeEach(func() {
								stateToken := signStateToken("example.com", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when redirecting to the ATC", func() {
							BeforeEach(func() {
								stateToken := signStateToken("/valid-redirect", time.Now().Unix())
								request.URL.RawQuery = "code=some-code&state=" + stateToken
							})

							Context("when setting the auth token fails", func() {
								BeforeEach(func() {
									fakeTokenMiddleware.SetAuthTokenReturns(errors.New("nope"))
								})
								It("errors", func() {
									Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})

							Context("when setting the auth token succeeds", func() {
								BeforeEach(func() {
									fakeTokenMiddleware.SetAuthTokenReturns(nil)
								})

								Context("when setting the csrf token fails", func() {
									BeforeEach(func() {
										fakeTokenMiddleware.SetCSRFTokenReturns(errors.New("nope"))
									})
									It("errors", func() {
										Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
									})
								})

								Context("when setting the csrf token succeeds", func() {
									BeforeEach(func() {
										fakeTokenMiddleware.SetCSRFTokenReturns(nil)
									})

									It("saves the access token from the response", func() {
										Expect(fakeTokenMiddleware.SetAuthTokenCallCount()).To(Equal(1))
										_, tokenString, _ := fakeTokenMiddleware.SetAuthTokenArgsForCall(0)
										Expect(tokenString).To(Equal("some-type some-token"))
									})

									It("sets a new csrf token", func() {
										Expect(fakeTokenMiddleware.SetCSRFTokenCallCount()).To(Equal(1))
										_, tokenString, _ := fakeTokenMiddleware.SetCSRFTokenArgsForCall(0)
										Expect(tokenString).NotTo(BeEmpty())
									})

									It("redirects to redirect_uri from state token with the csrf_token", func() {
										_, tokenArg, _ := fakeTokenMiddleware.SetCSRFTokenArgsForCall(0)

										redirectResponse := response.Request.Response
										Expect(redirectResponse).NotTo(BeNil())
										Expect(redirectResponse.StatusCode).To(Equal(http.StatusTemporaryRedirect))

										skyServerURL, err := url.Parse(skyServer.URL)
										Expect(err).NotTo(HaveOccurred())

										locationURL, err := redirectResponse.Location()
										Expect(err).NotTo(HaveOccurred())
										Expect(locationURL.Host).To(Equal(skyServerURL.Host))
										Expect(locationURL.Path).To(Equal("/valid-redirect"))
										Expect(locationURL.Query().Get("csrf_token")).To(Equal(tokenArg))
									})
								})
							})
						})
					})
				})
			})
		})
	}

	Describe("With TLS Server", func() {
		BeforeEach(func() {
			skyServer.StartTLS()
		})

		ExpectServerBehaviour()
	})

	Describe("Without TLS Server", func() {
		BeforeEach(func() {
			skyServer.Start()
		})

		ExpectServerBehaviour()
	})
})
