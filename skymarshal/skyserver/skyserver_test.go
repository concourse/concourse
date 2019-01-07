package skyserver_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/skymarshal/token"
	"github.com/onsi/gomega/ghttp"
	"golang.org/x/oauth2"
)

var _ = Describe("Sky Server API", func() {

	ExpectServerBehaviour := func() {
		Describe("GET /sky/login", func() {
			var params url.Values
			var response *http.Response
			var cookies []*http.Cookie
			var cookieValue string

			BeforeEach(func() {
				params = url.Values{}
				params.Add("redirect_uri", "http://example.com")
			})

			ExpectNewLogin := func() {
				It("stores a state cookie", func() {
					Expect(cookies[0].Name).To(Equal("skymarshal_state"))
					Expect(cookies[0].Secure).To(Equal(config.SecureCookies))
					Expect(cookies[0].HttpOnly).To(BeTrue())
					Expect(cookies[0].Value).NotTo(BeEmpty())
				})

				It("redirects the initial request to /sky/issuer/auth", func() {
					redirectURL, err := response.Location()
					Expect(err).NotTo(HaveOccurred())
					Expect(redirectURL.Path).To(Equal("/sky/issuer/auth"))

					redirectValues := redirectURL.Query()
					Expect(redirectValues.Get("access_type")).To(Equal("offline"))
					Expect(redirectValues.Get("response_type")).To(Equal("code"))
					Expect(redirectValues.Get("state")).To(Equal(cookies[0].Value))
					Expect(redirectValues.Get("scope")).To(Equal("openid profile email federated:id groups"))
				})

				Context("when redirect_uri is provided", func() {
					BeforeEach(func() {
						params.Add("redirect_uri", "http://example.com")
					})

					It("stores redirect_uri in the state token cookie", func() {
						data, err := base64.StdEncoding.DecodeString(cookies[0].Value)
						Expect(err).NotTo(HaveOccurred())

						var state map[string]string
						json.Unmarshal(data, &state)
						Expect(state["redirect_uri"]).To(Equal("http://example.com"))
					})
				})

				Context("when redirect_uri is NOT provided", func() {
					BeforeEach(func() {
						params.Del("redirect_uri")
					})

					It("stores / as the default redirect_uri in the state token cookie", func() {
						data, err := base64.StdEncoding.DecodeString(cookies[0].Value)
						Expect(err).NotTo(HaveOccurred())

						var state map[string]string
						json.Unmarshal(data, &state)
						Expect(state["redirect_uri"]).To(Equal("/"))
					})
				})
			}

			ExpectAlreadyLoggedIn := func() {
				It("redirects the request to the provided redirect_uri", func() {
					redirectURL, err := response.Location()
					Expect(err).NotTo(HaveOccurred())
					Expect(redirectURL.Host).To(Equal("example.com"))

					redirectValues := redirectURL.Query()
					Expect(redirectValues.Get("token")).To(Equal(""))
					Expect(redirectValues.Get("csrf_token")).To(Equal("some-csrf"))
				})

				It("doesn't modify the cookie", func() {
					Expect(cookies[0].Name).To(Equal("skymarshal_auth"))
					Expect(cookies[0].HttpOnly).To(BeTrue())
					Expect(cookies[0].Value).To(Equal(cookieValue))
				})
			}

			Context("without an existing auth cookie", func() {
				BeforeEach(func() {
					dexServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", "/sky/issuer/auth"),
							ghttp.VerifyFormKV("scope", "openid profile email federated:id groups"),
							ghttp.RespondWith(http.StatusTemporaryRedirect, nil, http.Header{
								"Location": {"http://example.com"},
							}),
						),
					)
				})

				JustBeforeEach(func() {
					client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					}

					request, err := http.NewRequest("GET", skyServer.URL+"/sky/login?"+params.Encode(), nil)
					Expect(err).NotTo(HaveOccurred())

					response, err = client.Do(request)
					Expect(err).NotTo(HaveOccurred())

					cookies = response.Cookies()
					Expect(cookies).To(HaveLen(1))
				})

				ExpectNewLogin()
			})

			Context("with an existing auth cookie", func() {
				var cookieExpiration time.Time

				JustBeforeEach(func() {
					client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					}

					request, err := http.NewRequest("GET", skyServer.URL+"/sky/login?"+params.Encode(), nil)
					Expect(err).NotTo(HaveOccurred())

					request.AddCookie(&http.Cookie{
						Name:     "skymarshal_auth",
						Value:    cookieValue,
						Path:     "/",
						Expires:  cookieExpiration,
						HttpOnly: true,
					})

					response, err = client.Do(request)
					Expect(err).NotTo(HaveOccurred())

					cookies = response.Cookies()
					Expect(cookies).To(HaveLen(1))
				})

				Context("which is not a valid bearer token", func() {
					BeforeEach(func() {
						cookieValue = "NotBearer some-token"
					})
					ExpectNewLogin()
				})

				Context("which is not a signed auth token", func() {
					BeforeEach(func() {
						cookieValue = "Bearer some-token"
					})
					ExpectNewLogin()
				})

				Context("which is an expired auth token", func() {
					BeforeEach(func() {
						cookieExpiration = time.Now().Add(-1 * time.Hour)

						tokenGenerator := token.NewGenerator(signingKey)
						oauthToken, err := tokenGenerator.Generate(map[string]interface{}{
							"exp":  cookieExpiration.Unix(),
							"csrf": "some-csrf",
						})
						Expect(err).NotTo(HaveOccurred())

						cookieValue = oauthToken.TokenType + " " + oauthToken.AccessToken
					})
					ExpectNewLogin()
				})

				Context("which is a valid auth token", func() {
					BeforeEach(func() {
						cookieExpiration = time.Now().Add(1 * time.Hour)

						tokenGenerator := token.NewGenerator(signingKey)
						oauthToken, err := tokenGenerator.Generate(map[string]interface{}{
							"exp":  cookieExpiration.Unix(),
							"csrf": "some-csrf",
						})
						Expect(err).NotTo(HaveOccurred())

						cookieValue = oauthToken.TokenType + " " + oauthToken.AccessToken
					})

					ExpectAlreadyLoggedIn()
				})
			})
		})

		Describe("GET /sky/logout", func() {
			It("removes auth token cookie", func() {
				skyURL, err := url.Parse(skyServer.URL)
				Expect(err).NotTo(HaveOccurred())

				cookieJar.SetCookies(skyURL, []*http.Cookie{
					{Name: "skymarshal_auth", Value: "some-cookie"},
				})
				Expect(cookieJar.Cookies(skyURL)).NotTo(BeEmpty())

				response, err := client.Get(skyServer.URL + "/sky/logout")
				Expect(err).NotTo(HaveOccurred())

				cookieResponse := response.Header.Get("Set-Cookie")
				Expect(strings.Contains(cookieResponse, "HttpOnly")).To(BeTrue())
				Expect(strings.Contains(cookieResponse, "Secure")).To(Equal(config.SecureCookies))

				Expect(cookieJar.Cookies(skyURL)).To(BeEmpty())
			})
		})

		Describe("GET /sky/callback", func() {
			var (
				err         error
				request     *http.Request
				response    *http.Response
				stateCookie *http.Cookie
				reqPath     string
			)
			BeforeEach(func() {
				reqPath = "/sky/callback?state=some-state&code=some-code"
				stateCookie = &http.Cookie{Name: "skymarshal_state", Value: "some-state"}
			})

			JustBeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+reqPath, nil)
				Expect(err).NotTo(HaveOccurred())

				skyURL, err := url.Parse(skyServer.URL)
				Expect(err).NotTo(HaveOccurred())

				cookieJar.SetCookies(skyURL, []*http.Cookie{stateCookie})

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("the state cookie doesn't exist", func() {
				BeforeEach(func() {
					stateCookie = &http.Cookie{}
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("dex returns an error param", func() {
				BeforeEach(func() {
					reqPath = "/sky/callback?error=some-error"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("the cookie state doesn't match the form state", func() {
				BeforeEach(func() {
					stateCookie = &http.Cookie{Name: "skymarshal_state", Value: "not-some-state"}
					reqPath = "/sky/callback?code=some-code&state=some-state"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("dex doesn't return an authorization code", func() {
				BeforeEach(func() {
					reqPath = "/sky/callback?state=some-state"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("requesting a token from dex fails", func() {
				BeforeEach(func() {
					dexServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/sky/issuer/token"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("requesting a token from dex succeeds", func() {
				var fakeVerifiedClaims *token.VerifiedClaims
				var fakeOAuthToken *oauth2.Token

				BeforeEach(func() {
					dexServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/sky/issuer/token"),
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

				Context("the token verification fails", func() {
					BeforeEach(func() {
						fakeTokenVerifier.VerifyReturns(nil, errors.New("error"))
					})

					It("passes the correct args to the token verifier", func() {
						_, dexToken := fakeTokenVerifier.VerifyArgsForCall(0)
						Expect(dexToken.AccessToken).To(Equal("some-token"))
						Expect(dexToken.TokenType).To(Equal("some-type"))
						Expect(dexToken.Extra("id_token")).To(Equal("some-id-token"))
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("issuing the concourse token fails", func() {
					BeforeEach(func() {
						fakeVerifiedClaims = &token.VerifiedClaims{}
						fakeTokenVerifier.VerifyReturns(fakeVerifiedClaims, nil)
						fakeTokenIssuer.IssueReturns(nil, errors.New("error"))
					})

					It("passes the correct args to the token issuer", func() {
						idToken := fakeTokenIssuer.IssueArgsForCall(0)
						Expect(idToken).To(Equal(fakeVerifiedClaims))
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("the request succeeds", func() {
					BeforeEach(func() {
						fakeVerifiedClaims = &token.VerifiedClaims{}

						fakeOAuthToken = (&oauth2.Token{
							TokenType:   "some-type",
							AccessToken: "some-token",
							Expiry:      time.Now().Add(time.Minute),
						}).WithExtra(map[string]interface{}{
							"csrf": "some-csrf",
						})

						fakeTokenVerifier.VerifyReturns(fakeVerifiedClaims, nil)
						fakeTokenIssuer.IssueReturns(fakeOAuthToken, nil)

						state, _ := json.Marshal(map[string]string{
							"redirect_uri": "http://example.com",
						})

						stateToken := base64.StdEncoding.EncodeToString(state)
						stateCookie = &http.Cookie{Name: "skymarshal_state", Value: stateToken}
						reqPath = "/sky/callback?code=some-code&state=" + stateToken
					})

					It("only has one cookie containing the auth token (state cookie is gone)", func() {
						serverURL, err := url.Parse(skyServer.URL)
						Expect(err).NotTo(HaveOccurred())

						cookies := cookieJar.Cookies(serverURL)
						Expect(cookies).To(HaveLen(1))

						authCookie := cookies[0]
						Expect(authCookie.Name).To(Equal("skymarshal_auth"))
						Expect(authCookie.Value).To(Equal("some-type some-token"))
					})

					It("redirects to redirect_uri provided in the stateToken", func() {
						redirectResponse := response.Request.Response
						Expect(redirectResponse).NotTo(BeNil())
						Expect(redirectResponse.StatusCode).To(Equal(http.StatusTemporaryRedirect))

						locationURL, err := redirectResponse.Location()
						Expect(err).NotTo(HaveOccurred())
						Expect(locationURL.String()).To(Equal("http://example.com?csrf_token=some-csrf"))
					})
				})
			})
		})

		Describe("PUT /sky/token", func() {
			var (
				err      error
				request  *http.Request
				response *http.Response
			)

			JustBeforeEach(func() {
				reqPayload := "grant_type=password&username=some-username&password=some-password&scope=some-scope"

				request, err = http.NewRequest("PUT", skyServer.URL+"/sky/token?"+reqPayload, nil)
				request.Header.Add("Authorization", "Basic "+string(base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))))
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			It("rejects every request", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Describe("GET /sky/token", func() {
			var (
				err      error
				request  *http.Request
				response *http.Response
			)

			JustBeforeEach(func() {
				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when authenticated", func() {
				var oauthToken *oauth2.Token

				BeforeEach(func() {
					request, err = http.NewRequest("GET", skyServer.URL+"/sky/token", nil)
					Expect(err).NotTo(HaveOccurred())

					cookieExpiration := time.Now().Add(time.Hour)
					tokenGenerator := token.NewGenerator(signingKey)
					oauthToken, err = tokenGenerator.Generate(map[string]interface{}{
						"exp":  cookieExpiration.Unix(),
						"csrf": "some-csrf",
					})
					Expect(err).NotTo(HaveOccurred())

					request.AddCookie(&http.Cookie{
						Name:     "skymarshal_auth",
						Value:    oauthToken.TokenType + " " + oauthToken.AccessToken,
						Path:     "/",
						Expires:  cookieExpiration,
						HttpOnly: true,
					})
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the concourse token", func() {
					token, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(token)).To(Equal(oauthToken.TokenType + " " + oauthToken.AccessToken))
				})
			})

			Context("when not authenticated", func() {
				BeforeEach(func() {
					request, err = http.NewRequest("GET", skyServer.URL+"/sky/token", nil)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})

		Describe("POST /sky/token", func() {
			var (
				err        error
				request    *http.Request
				response   *http.Response
				reqHeader  http.Header
				reqPayload string
			)

			BeforeEach(func() {
				reqPayload = "grant_type=password&username=some-username&password=some-password&scope=some-scope"

				reqHeader = http.Header{}
				reqHeader.Set("Authorization", "Basic "+string(base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))))
				reqHeader.Set("Content-Type", "application/x-www-form-urlencoded")
			})

			JustBeforeEach(func() {
				request, err = http.NewRequest("POST", skyServer.URL+"/sky/token", strings.NewReader(reqPayload))
				request.Header = reqHeader
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when missing authorization header", func() {
				BeforeEach(func() {
					reqHeader.Del("Authorization")
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("when authorization header is not basic auth", func() {
				BeforeEach(func() {
					reqHeader.Set("Authorization", "Bearer token")
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("when authorization header is not of the form 'client_id:client_secret'", func() {
				BeforeEach(func() {
					credentials := base64.StdEncoding.EncodeToString([]byte("some-string"))
					reqHeader.Set("Authorization", "Basic "+string(credentials))
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("when not using the public fly client id", func() {
				BeforeEach(func() {
					credentials := base64.StdEncoding.EncodeToString([]byte("not-fly:Zmx5"))
					reqHeader.Set("Authorization", "Basic "+string(credentials))
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("when not using the public fly client secret", func() {
				BeforeEach(func() {
					credentials := base64.StdEncoding.EncodeToString([]byte("fly:not-fly-secret"))
					reqHeader.Set("Authorization", "Basic "+string(credentials))
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("payload is malformed", func() {

				Context("grant type is not 'password'", func() {
					BeforeEach(func() {
						reqPayload = "grant_type=client_credentials&username=some-username&password=some-password&scope=some-scope"
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("username is missing", func() {
					BeforeEach(func() {
						reqPayload = "grant_type=client_credentials&password=some-password&scope=some-scope"
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("password is missing", func() {
					BeforeEach(func() {
						reqPayload = "grant_type=client_credentials&username=some-username&scope=some-scope"
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("scope is missing", func() {
					BeforeEach(func() {
						reqPayload = "grant_type=client_credentials&username=some-username&password=some-password"
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})
			})

			Context("requesting a token from dex fails", func() {
				BeforeEach(func() {
					dexServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/sky/issuer/token"),
							ghttp.RespondWith(http.StatusInternalServerError, nil),
						),
					)
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("requesting a token from dex succeeds", func() {
				var fakeVerifiedClaims *token.VerifiedClaims
				var fakeOAuthToken *oauth2.Token

				BeforeEach(func() {
					dexServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("POST", "/sky/issuer/token"),
							ghttp.VerifyHeaderKV("Authorization", "Basic ZGV4LWNsaWVudC1pZDpkZXgtY2xpZW50LXNlY3JldA=="),
							ghttp.VerifyFormKV("grant_type", "password"),
							ghttp.VerifyFormKV("username", "some-username"),
							ghttp.VerifyFormKV("password", "some-password"),
							ghttp.VerifyFormKV("scope", "some-scope"),
							ghttp.RespondWithJSONEncoded(http.StatusOK, map[string]string{
								"token_type":   "some-type",
								"access_token": "some-token",
								"id_token":     "some-id-token",
							}),
						),
					)
				})

				Context("the token verification fails", func() {
					BeforeEach(func() {
						fakeTokenVerifier.VerifyReturns(nil, errors.New("error"))
					})

					It("passes the correct args to the token verifier", func() {
						_, dexToken := fakeTokenVerifier.VerifyArgsForCall(0)
						Expect(dexToken.AccessToken).To(Equal("some-token"))
						Expect(dexToken.TokenType).To(Equal("some-type"))
						Expect(dexToken.Extra("id_token")).To(Equal("some-id-token"))
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("issuing the concourse token fails", func() {
					BeforeEach(func() {
						fakeVerifiedClaims = &token.VerifiedClaims{}
						fakeTokenVerifier.VerifyReturns(fakeVerifiedClaims, nil)
						fakeTokenIssuer.IssueReturns(nil, errors.New("error"))
					})

					It("passes the correct args to the token issuer", func() {
						idToken := fakeTokenIssuer.IssueArgsForCall(0)
						Expect(idToken).To(Equal(fakeVerifiedClaims))
					})

					It("errors", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("the request succeeds", func() {
					BeforeEach(func() {
						fakeVerifiedClaims = &token.VerifiedClaims{}

						fakeOAuthToken = &oauth2.Token{
							TokenType:   "some-type",
							AccessToken: "some-token",
						}

						fakeTokenVerifier.VerifyReturns(fakeVerifiedClaims, nil)
						fakeTokenIssuer.IssueReturns(fakeOAuthToken, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns application/json", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns the concourse token", func() {
						var token map[string]string
						err := json.NewDecoder(response.Body).Decode(&token)
						Expect(err).NotTo(HaveOccurred())

						Expect(token["token_type"]).To(Equal(fakeOAuthToken.TokenType))
						Expect(token["access_token"]).To(Equal(fakeOAuthToken.AccessToken))
					})
				})
			})
		})

		Describe("GET /sky/userinfo", func() {
			var (
				err       error
				request   *http.Request
				response  *http.Response
				reqHeader http.Header
			)

			BeforeEach(func() {
				reqHeader = http.Header{}
			})

			JustBeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/userinfo", nil)
				request.Header = reqHeader
				Expect(err).NotTo(HaveOccurred())

				response, err = client.Do(request)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("missing authorization header", func() {
				BeforeEach(func() {
					reqHeader.Del("Authorization")
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("authorization header is not a bearer token", func() {
				BeforeEach(func() {
					reqHeader.Set("Authorization", "Basic some-token")
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("bearer token is not valid", func() {
				BeforeEach(func() {
					reqHeader.Set("Authorization", "Bearer some-invalid-token")
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("bearer token is signed with the wrong key", func() {
				BeforeEach(func() {
					wrongSigningKey, err := rsa.GenerateKey(rand.Reader, 2048)
					Expect(err).NotTo(HaveOccurred())

					tokenGenerator := token.NewGenerator(wrongSigningKey)
					token, err := tokenGenerator.Generate(map[string]interface{}{"sub": "some-sub"})
					Expect(err).NotTo(HaveOccurred())

					reqHeader.Set("Authorization", token.TokenType+" "+token.AccessToken)
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("bearer token is expired", func() {
				BeforeEach(func() {
					tokenGenerator := token.NewGenerator(signingKey)
					token, err := tokenGenerator.Generate(map[string]interface{}{
						"exp": time.Now().Add(-1 * time.Hour).Unix(),
					})
					Expect(err).NotTo(HaveOccurred())

					reqHeader.Set("Authorization", token.TokenType+" "+token.AccessToken)
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("bearer token is valid", func() {
				var expiration int64

				BeforeEach(func() {
					expiration = time.Now().Add(1 * time.Hour).Unix()

					tokenGenerator := token.NewGenerator(signingKey)
					token, err := tokenGenerator.Generate(map[string]interface{}{
						"exp":       expiration,
						"sub":       "some-sub",
						"user_id":   "some-user-id",
						"user_name": "some-user-name",
						"teams":     []string{"some-team"},
						"csrf":      "some-csrf",
						"is_admin":  true,
					})
					Expect(err).NotTo(HaveOccurred())

					reqHeader.Set("Authorization", token.TokenType+" "+token.AccessToken)
				})

				It("succeeds", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("outputs the claims from the token", func() {
					var token map[string]interface{}
					err := json.NewDecoder(response.Body).Decode(&token)
					Expect(err).NotTo(HaveOccurred())

					Expect(token["exp"]).To(Equal(float64(expiration)))
					Expect(token["sub"]).To(Equal("some-sub"))
					Expect(token["user_id"]).To(Equal("some-user-id"))
					Expect(token["user_name"]).To(Equal("some-user-name"))
					Expect(token["teams"]).To(ContainElement("some-team"))
					Expect(token["csrf"]).To(Equal("some-csrf"))
					Expect(token["is_admin"]).To(Equal(true))
				})
			})
		})
	}

	Describe("With TLS Server", func() {
		BeforeEach(func() {
			config.SecureCookies = true
			skyServer.StartTLS()
		})

		ExpectServerBehaviour()
	})

	Describe("Without TLS Server", func() {
		BeforeEach(func() {
			config.SecureCookies = false
			skyServer.Start()
		})

		ExpectServerBehaviour()
	})
})
