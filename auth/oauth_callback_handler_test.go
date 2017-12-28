package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"regexp"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/auth/authfakes"
	"github.com/concourse/skymarshal/provider"
	"github.com/concourse/skymarshal/provider/providerfakes"
)

type testCookieJar struct {
	cookies []*http.Cookie
}

func newCookieJar() *testCookieJar {
	cookies := make([]*http.Cookie, 0, 10)
	return &testCookieJar{
		cookies: cookies,
	}
}

func (j *testCookieJar) Cookies(_ *url.URL) (_ []*http.Cookie) {
	return j.cookies
}

func (j *testCookieJar) SetCookies(_ *url.URL, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		j.cookies = append(j.cookies, cookie)
	}
}

var _ = Describe("OAuthCallbackHandler", func() {
	var (
		fakeProvider   *providerfakes.FakeProvider
		preTokenClient *http.Client

		fakeProviderFactory    *authfakes.FakeProviderFactory
		fakeCSRFTokenGenerator *authfakes.FakeCSRFTokenGenerator
		fakeAuthTokenGenerator *authfakes.FakeAuthTokenGenerator

		fakeTeam        *dbfakes.FakeTeam
		fakeTeamFactory *dbfakes.FakeTeamFactory

		expire time.Duration

		server          *httptest.Server
		client          *http.Client
		redirectRequest *http.Request
	)

	BeforeEach(func() {
		fakeProvider = new(providerfakes.FakeProvider)

		fakeTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeProviderFactory = new(authfakes.FakeProviderFactory)
		fakeCSRFTokenGenerator = new(authfakes.FakeCSRFTokenGenerator)
		fakeAuthTokenGenerator = new(authfakes.FakeAuthTokenGenerator)

		fakeAuthTokenGenerator.GenerateTokenReturns("TOKEN_TYPE", "token", nil)
		fakeCSRFTokenGenerator.GenerateTokenReturns("CSRF_TOKEN", nil)
		expire = 24 * time.Hour

		fakeProviderFactory.GetProviderStub = func(team db.Team, providerName string) (provider.Provider, bool, error) {
			if providerName == "some-provider" {
				return fakeProvider, true, nil
			}
			return nil, false, nil
		}

		preTokenClient = &http.Client{Timeout: 31 * time.Second}
		fakeProvider.PreTokenClientReturns(preTokenClient, nil)

		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
		fakeTeam.NameReturns("some-team")

		handler, err := auth.NewOAuthHandler(
			lagertest.NewTestLogger("test"),
			fakeProviderFactory,
			fakeTeamFactory,
			fakeCSRFTokenGenerator,
			fakeAuthTokenGenerator,
			expire,
			false,
		)
		Expect(err).ToNot(HaveOccurred())

		mux := http.NewServeMux()
		mux.Handle("/auth/", handler)
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			redirectRequest = r
			fmt.Fprintln(w, "main page")
		}))
		mux.Handle("/public/fly_success", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "fly success page")
		}))

		server = httptest.NewServer(mux)
		jar := newCookieJar()
		client = &http.Client{
			Transport: &http.Transport{},
			Jar:       jar,
		}
	})

	Describe("GET /auth/:provider/callback", func() {
		var redirectTarget *ghttp.Server
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			redirectTarget = ghttp.NewServer()
			redirectTarget.RouteToHandler("GET", "/", ghttp.RespondWith(http.StatusOK, "sup"))

			var err error

			request, err = http.NewRequest("GET", server.URL, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("to a known provider", func() {
			BeforeEach(func() {
				request.URL.Path = "/auth/some-provider/callback"
			})

			Context("when the request's state is valid", func() {
				BeforeEach(func() {

					flyTarget := ghttp.NewServer()
					headers := map[string][]string{
						"Location": []string{fmt.Sprintf("%s://%s/public/fly_success", request.URL.Scheme, request.URL.Host)},
					}

					flyTarget.AppendHandlers(ghttp.RespondWith(http.StatusTemporaryRedirect, "", headers))

					r, err := regexp.Compile(".*:(\\d+)")
					Expect(err).ToNot(HaveOccurred())

					port := r.FindStringSubmatch(flyTarget.Addr())[1]
					state, err := json.Marshal(auth.OAuthState{
						TeamName:     "some-team",
						FlyLocalPort: port,
					})
					encodedState := base64.RawURLEncoding.EncodeToString(state)

					request.AddCookie(&http.Cookie{
						Name:    auth.OAuthStateCookie,
						Value:   encodedState,
						Path:    "/",
						Expires: time.Now().Add(time.Hour),
					})

					request.URL.RawQuery = url.Values{
						"code":  {"some-code"},
						"state": {encodedState},
					}.Encode()
				})

				Context("when exchanging the token succeeds", func() {
					var token *oauth2.Token
					var httpClient *http.Client

					BeforeEach(func() {
						token = &oauth2.Token{AccessToken: "some-access-token"}
						httpClient = &http.Client{}

						fakeProvider.ExchangeReturns(token, nil)
						fakeProvider.ClientReturns(httpClient)
					})

					It("generated the OAuth token using the request's code", func() {
						Expect(fakeProvider.ExchangeCallCount()).To(Equal(1))
						_, request := fakeProvider.ExchangeArgsForCall(0)
						Expect(request.FormValue("code")).To(Equal("some-code"))
					})

					It("uses the PreTokenClient from the provider", func() {
						ctx, _ := fakeProvider.ClientArgsForCall(0)
						actualPreTokenClient, ok := ctx.Value(oauth2.HTTPClient).(*http.Client)
						Expect(ok).To(BeTrue())
						Expect(actualPreTokenClient).To(Equal(preTokenClient))
					})

					It("looks up the verifier for the team from the query param", func() {
						Expect(fakeProviderFactory.GetProviderCallCount()).To(Equal(1))
						argTeam, providerName := fakeProviderFactory.GetProviderArgsForCall(0)
						Expect(argTeam).To(Equal(fakeTeam))
						Expect(providerName).To(Equal("some-provider"))
					})

					Context("when the token is verified", func() {
						BeforeEach(func() {
							fakeProvider.VerifyReturns(true, nil)
						})

						It("responds OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("verifies using the provider's HTTP client", func() {
							Expect(fakeProvider.ClientCallCount()).To(Equal(1))
							_, clientToken := fakeProvider.ClientArgsForCall(0)
							Expect(clientToken).To(Equal(token))

							Expect(fakeProvider.VerifyCallCount()).To(Equal(1))
							_, client := fakeProvider.VerifyArgsForCall(0)
							Expect(client).To(Equal(httpClient))
						})

						Describe("the ATC-Authorization cookie", func() {
							var cookie *http.Cookie

							JustBeforeEach(func() {
								cookies := client.Jar.Cookies(request.URL)
								for _, c := range cookies {
									if c.Name == auth.AuthCookieName {
										cookie = c
									}
								}
							})

							It("set to a signed token that expires in 1 day", func() {
								Expect(cookie).NotTo(BeNil())
								Expect(cookie.Expires).To(BeTemporally("~", time.Now().Add(24*time.Hour), 5*time.Second))
							})

							It("stores the value of the token provided by the generator", func() {
								Expect(cookie.Value).To(Equal(`TOKEN_TYPE token`))
							})
						})

						It("does not redirect", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("responds with the success page and deletes oauth state cookie", func() {
							cookies := client.Jar.Cookies(request.URL)
							Expect(cookies).To(HaveLen(2))

							var authCookie *http.Cookie
							var oauthStateCookie *http.Cookie

							for _, c := range cookies {
								if c.Name == auth.AuthCookieName {
									authCookie = c
								}
								if c.Name == auth.OAuthStateCookie {
									oauthStateCookie = c
								}
							}
							Expect(authCookie).NotTo(BeNil())
							Expect(authCookie.Value).To(Equal(`TOKEN_TYPE token`))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("fly success page\n")))

							Expect(oauthStateCookie).NotTo(BeNil())
							Expect(oauthStateCookie.MaxAge).To(Equal(-1))
						})
					})

					Context("when the token is not verified", func() {
						BeforeEach(func() {
							fakeProvider.VerifyReturns(false, nil)
						})

						It("returns Unauthorized", func() {
							Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
						})

						It("does not set a cookie", func() {
							Expect(response.Cookies()).To(BeEmpty())
						})
					})

					Context("when the token cannot be verified", func() {
						BeforeEach(func() {
							fakeProvider.VerifyReturns(false, errors.New("nope"))
						})

						It("returns Internal Server Error", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})

						It("does not set a cookie", func() {
							Expect(response.Cookies()).To(BeEmpty())
						})
					})
				})

				Context("when the team cannot be found", func() {
					BeforeEach(func() {
						fakeTeamFactory.FindTeamReturns(nil, false, nil)
					})

					It("returns Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})

					It("does not set a cookie", func() {
						Expect(response.Cookies()).To(BeEmpty())
					})

					It("does not set exchange the token", func() {
						Expect(fakeProvider.ExchangeCallCount()).To(Equal(0))
					})
				})
			})

			Context("when a redirect URI is in the state", func() {
				Context("when the redirect URI is external", func() {
					BeforeEach(func() {
						state, err := json.Marshal(auth.OAuthState{
							Redirect: "https://google.com",
							TeamName: "some-team",
						})
						Expect(err).ToNot(HaveOccurred())

						encodedState := base64.RawURLEncoding.EncodeToString(state)

						request.AddCookie(&http.Cookie{
							Name:    auth.OAuthStateCookie,
							Value:   encodedState,
							Path:    "/",
							Expires: time.Now().Add(time.Hour),
						})

						request.URL.RawQuery = url.Values{
							"code":  {"some-code"},
							"state": {encodedState},
						}.Encode()
						fakeProvider.VerifyReturns(true, nil)

					})
					It("does not redirect", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})
				})

				Context("when the redirect URI is not external", func() {
					BeforeEach(func() {
						state, err := json.Marshal(auth.OAuthState{
							Redirect: "/",
						})
						Expect(err).ToNot(HaveOccurred())

						encodedState := base64.RawURLEncoding.EncodeToString(state)

						request.AddCookie(&http.Cookie{
							Name:    auth.OAuthStateCookie,
							Value:   encodedState,
							Path:    "/",
							Expires: time.Now().Add(time.Hour),
						})

						request.URL.RawQuery = url.Values{
							"code":  {"some-code"},
							"state": {encodedState},
						}.Encode()

					})

					Context("when exchanging the token succeeds", func() {
						var token *oauth2.Token
						var httpClient *http.Client

						BeforeEach(func() {
							token = &oauth2.Token{AccessToken: "some-access-token"}
							httpClient = &http.Client{}

							fakeProvider.ExchangeReturns(token, nil)
							fakeProvider.ClientReturns(httpClient)
						})

						It("generated the OAuth token using the request's code", func() {
							Expect(fakeProvider.ExchangeCallCount()).To(Equal(1))
							_, request := fakeProvider.ExchangeArgsForCall(0)
							Expect(request.FormValue("code")).To(Equal("some-code"))
						})

						Context("when the token is verified", func() {
							BeforeEach(func() {
								fakeProvider.VerifyReturns(true, nil)
							})

							It("redirects to the redirect uri", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("main page\n")))
							})

							It("appends csrf token to redirect request", func() {
								Expect(redirectRequest.URL.RawQuery).To(MatchRegexp("csrf_token=CSRF_TOKEN"))
							})
						})

						Context("when the token is not verified", func() {
							BeforeEach(func() {
								fakeProvider.VerifyReturns(false, nil)
							})

							It("returns Unauthorized", func() {
								Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
							})

							It("does not set a cookie", func() {
								Expect(response.Cookies()).To(BeEmpty())
							})
						})

						Context("when the token cannot be verified", func() {
							BeforeEach(func() {
								fakeProvider.VerifyReturns(false, errors.New("nope"))
							})

							It("returns Internal Server Error", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})

							It("does not set a cookie", func() {
								Expect(response.Cookies()).To(BeEmpty())
							})
						})
					})
				})

			})

			Context("when the request has no state", func() {
				BeforeEach(func() {
					request.URL.RawQuery = url.Values{
						"code": {"some-code"},
					}.Encode()
				})

				It("returns Unauthorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not set a cookie", func() {
					Expect(response.Cookies()).To(BeEmpty())
				})

				It("does not set exchange the token", func() {
					Expect(fakeProvider.ExchangeCallCount()).To(Equal(0))
				})
			})

			Context("when the request's state is bogus", func() {
				BeforeEach(func() {
					request.URL.RawQuery = url.Values{
						"code":  {"some-code"},
						"state": {"bogus-state"},
					}.Encode()
				})

				It("returns Unauthorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not set a cookie", func() {
					Expect(response.Cookies()).To(BeEmpty())
				})

				It("does not set exchange the token", func() {
					Expect(fakeProvider.ExchangeCallCount()).To(Equal(0))
				})
			})

			Context("when the request's state is not set as a cookie", func() {
				BeforeEach(func() {
					state, err := json.Marshal(auth.OAuthState{})
					Expect(err).ToNot(HaveOccurred())

					encodedState := base64.RawURLEncoding.EncodeToString(state)

					request.URL.RawQuery = url.Values{
						"code":  {"some-code"},
						"state": {encodedState},
					}.Encode()
				})

				It("returns Unauthorized", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})

				It("does not set a cookie", func() {
					Expect(response.Cookies()).To(BeEmpty())
				})

				It("does not set exchange the token", func() {
					Expect(fakeProvider.ExchangeCallCount()).To(Equal(0))
				})
			})
		})

		Context("to an unknown provider", func() {
			BeforeEach(func() {
				request.URL.Path = "/auth/bogus/callback"
			})

			Context("when the request's state is valid", func() {
				BeforeEach(func() {
					state, err := json.Marshal(auth.OAuthState{
						TeamName: "some-team",
					})
					Expect(err).ToNot(HaveOccurred())

					encodedState := base64.RawURLEncoding.EncodeToString(state)

					request.AddCookie(&http.Cookie{
						Name:    auth.OAuthStateCookie,
						Value:   encodedState,
						Path:    "/",
						Expires: time.Now().Add(time.Hour),
					})

					request.URL.RawQuery = url.Values{
						"code":  {"some-code"},
						"state": {encodedState},
					}.Encode()
				})

				It("returns Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})
})
