package skyserver_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/skymarshal/token"
	"github.com/onsi/gomega/ghttp"
	"golang.org/x/oauth2"
)

var _ = Describe("Sky Server API", func() {

	Describe("GET /sky/login", func() {
		var err error
		var response *http.Response
		var cookies []*http.Cookie

		BeforeEach(func() {
			dexServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/sky/dex/auth"),
					ghttp.VerifyFormKV("scope", "openid profile email federated:id groups"),
					ghttp.RespondWith(http.StatusTemporaryRedirect, nil, http.Header{
						"Location": {"http://example.com"},
					}),
				),
			)

			response, err = client.Get(skyServer.URL + "/sky/login?redirect_uri=http://example.com")
			Expect(err).NotTo(HaveOccurred())

			serverURL, err := url.Parse(skyServer.URL)
			Expect(err).NotTo(HaveOccurred())

			cookies = cookieJar.Cookies(serverURL)
			Expect(cookies).To(HaveLen(1))
		})

		It("stores state token with redirect_uri in cookie", func() {
			Expect(cookies[0].Name).To(Equal("skymarshal_state"))

			data, err := base64.StdEncoding.DecodeString(cookies[0].Value)
			Expect(err).NotTo(HaveOccurred())

			var state struct {
				RedirectUri string `json:"redirect_uri"`
			}

			json.Unmarshal(data, &state)
			Expect(state.RedirectUri).To(Equal("http://example.com"))
		})

		It("redirects to /sky/dex/auth", func() {
			redirectUrl := response.Request.Response.Request.URL
			Expect(redirectUrl.Path).To(Equal("/sky/dex/auth"))

			redirectValues := redirectUrl.Query()
			Expect(redirectValues.Get("access_type")).To(Equal("offline"))
			Expect(redirectValues.Get("response_type")).To(Equal("code"))
			Expect(redirectValues.Get("state")).To(Equal(cookies[0].Value))
			Expect(redirectValues.Get("scope")).To(Equal("openid profile email federated:id groups"))
		})
	})

	Describe("GET /sky/logout", func() {
		It("removes auth token cookie", func() {
			skyUrl, err := url.Parse(skyServer.URL)
			Expect(err).NotTo(HaveOccurred())

			cookieJar.SetCookies(skyUrl, []*http.Cookie{
				{Name: "skymarshal_auth", Value: "some-cookie"},
			})
			Expect(cookieJar.Cookies(skyUrl)).NotTo(BeEmpty())

			_, err = client.Get(skyServer.URL + "/sky/logout")
			Expect(err).NotTo(HaveOccurred())

			Expect(cookieJar.Cookies(skyUrl)).To(BeEmpty())
		})
	})

	Describe("GET /sky/callback", func() {
		var (
			err         error
			request     *http.Request
			response    *http.Response
			stateCookie *http.Cookie
		)
		BeforeEach(func() {
			stateCookie = &http.Cookie{Name: "skymarshal_state", Value: "some-state"}

			request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback?state=some-state&code=some-code", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			skyUrl, err := url.Parse(skyServer.URL)
			Expect(err).NotTo(HaveOccurred())

			cookieJar.SetCookies(skyUrl, []*http.Cookie{stateCookie})

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("the state cookie doens't exist", func() {
			BeforeEach(func() {
				stateCookie = &http.Cookie{}
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("dex returns an error param", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback?error=some-error", nil)
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("the cookie state doesn't match the form state", func() {
			BeforeEach(func() {
				stateCookie = &http.Cookie{Name: "skymarshal_state", Value: "not-some-state"}
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback?code=some-code&state=some-state", nil)
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("dex doesn't return an authorization code", func() {
			BeforeEach(func() {
				request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback?state=some-state", nil)
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("requesting a token from dex fails", func() {
			BeforeEach(func() {
				dexServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/sky/dex/token"),
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
						ghttp.VerifyRequest("POST", "/sky/dex/token"),
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
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
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
					request, err = http.NewRequest("GET", skyServer.URL+"/sky/callback?code=some-code&state="+stateToken, nil)
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

					locationUrl, err := redirectResponse.Location()
					Expect(err).NotTo(HaveOccurred())
					Expect(locationUrl.String()).To(Equal("http://example.com?csrf_token=some-csrf&token=some-type+some-token"))
				})
			})
		})
	})

	Describe("POST /sky/token", func() {
		var (
			err      error
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			payload := "grant_type=password&username=some-username&password=some-password&scope=some-scope"
			credentials := base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))

			request, err = http.NewRequest("POST", skyServer.URL+"/sky/token", strings.NewReader(payload))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Authorization", "Basic "+string(credentials))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("missing authorization header", func() {
			BeforeEach(func() {
				request.Header.Del("Authorization")
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("authorization header is not basic auth", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Bearer token")
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("authorization header is not of the form 'client_id:client_secret'", func() {
			BeforeEach(func() {
				credentials := base64.StdEncoding.EncodeToString([]byte("some-string"))
				request.Header.Set("Authorization", "Basic "+string(credentials))
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("authorization header fails if not using the public fly client secret", func() {
			BeforeEach(func() {
				credentials := base64.StdEncoding.EncodeToString([]byte("fly:not-fly-secret"))
				request.Header.Set("Authorization", "Basic "+string(credentials))
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("authorization header fails if not using the public fly client id", func() {
			BeforeEach(func() {
				credentials := base64.StdEncoding.EncodeToString([]byte("not-fly:Zmx5"))
				request.Header.Set("Authorization", "Basic "+string(credentials))
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("payload is malformed", func() {
			var payload string

			BeforeEach(func() {
				credentials := base64.StdEncoding.EncodeToString([]byte("fly:Zmx5"))

				request, err = http.NewRequest("POST", skyServer.URL+"/sky/token", strings.NewReader(payload))
				request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				request.Header.Set("Authorization", "Basic "+string(credentials))
				Expect(err).NotTo(HaveOccurred())
			})

			Context("grant type is not 'password'", func() {
				BeforeEach(func() {
					payload = "grant_type=client_credentials&username=some-username&password=some-password&scope=some-scope"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("username is missing", func() {
				BeforeEach(func() {
					payload = "grant_type=client_credentials&password=some-password&scope=some-scope"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("password is missing", func() {
				BeforeEach(func() {
					payload = "grant_type=client_credentials&username=some-username&scope=some-scope"
				})

				It("errors", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})

			Context("scope is missing", func() {
				BeforeEach(func() {
					payload = "grant_type=client_credentials&username=some-username&password=some-password"
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
						ghttp.VerifyRequest("POST", "/sky/dex/token"),
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
						ghttp.VerifyRequest("POST", "/sky/dex/token"),
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
			err      error
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			tokenGenerator := token.NewGenerator(signingKey)
			token, err := tokenGenerator.Generate(map[string]interface{}{
				"sub":       "some-sub",
				"user_id":   "some-user-id",
				"user_name": "some-user-name",
				"teams":     []string{"some-team"},
				"csrf":      "some-csrf",
				"exp":       "32503680000",
				"is_admin":  true,
			})

			request, err = http.NewRequest("GET", skyServer.URL+"/sky/userinfo", nil)
			request.Header.Set("Authorization", token.TokenType+" "+token.AccessToken)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("missing authorization header", func() {
			BeforeEach(func() {
				request.Header.Del("Authorization")
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("authorization header is not a bearer token", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Basic some-token")
			})

			It("errors", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("the request succeeds", func() {
			It("outputs the claims from the token", func() {
				var token map[string]interface{}
				err := json.NewDecoder(response.Body).Decode(&token)
				Expect(err).NotTo(HaveOccurred())

				Expect(token["sub"]).To(Equal("some-sub"))
				Expect(token["user_id"]).To(Equal("some-user-id"))
				Expect(token["user_name"]).To(Equal("some-user-name"))
				Expect(token["teams"]).To(ContainElement("some-team"))
				Expect(token["is_admin"]).To(Equal(true))
				Expect(token["exp"]).To(Equal("32503680000"))
				Expect(token["csrf"]).To(Equal("some-csrf"))
			})
		})
	})
})
