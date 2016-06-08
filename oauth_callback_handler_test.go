package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/fakes"
	"github.com/concourse/atc/auth/provider"
	providerFakes "github.com/concourse/atc/auth/provider/fakes"
	"github.com/concourse/atc/db"
)

var _ = Describe("OAuthCallbackHandler", func() {
	var (
		fakeProviderA *providerFakes.FakeProvider
		fakeProviderB *providerFakes.FakeProvider

		fakeProviderFactory *fakes.FakeProviderFactory

		fakeAuthDB *fakes.FakeAuthDB

		signingKey *rsa.PrivateKey

		server *httptest.Server
		client *http.Client

		team db.SavedTeam
	)

	BeforeEach(func() {
		fakeProviderA = new(providerFakes.FakeProvider)
		fakeProviderB = new(providerFakes.FakeProvider)

		fakeProviderFactory = new(fakes.FakeProviderFactory)

		fakeAuthDB = new(fakes.FakeAuthDB)

		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())

		fakeProviderFactory.GetProvidersReturns(
			provider.Providers{
				"a": fakeProviderA,
				"b": fakeProviderB,
			},
			nil,
		)

		handler, err := auth.NewOAuthHandler(
			lagertest.NewTestLogger("test"),
			fakeProviderFactory,
			signingKey,
			fakeAuthDB,
		)
		Expect(err).ToNot(HaveOccurred())

		mux := http.NewServeMux()
		mux.Handle("/auth/", handler)
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "main page")
		}))

		server = httptest.NewServer(mux)

		client = &http.Client{
			Transport: &http.Transport{},
		}

		team = db.SavedTeam{
			ID: 0,
			Team: db.Team{
				Name: atc.DefaultTeamName,
			},
		}

		fakeAuthDB.GetTeamByNameReturns(team, true, nil)
	})

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return signingKey.Public(), nil
	}

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
				request.URL.Path = "/auth/b/callback"
			})

			Context("when the request's state is valid", func() {
				BeforeEach(func() {
					state, err := json.Marshal(auth.OAuthState{})
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

						fakeProviderB.ExchangeReturns(token, nil)
						fakeProviderB.ClientReturns(httpClient)
					})

					It("generated the OAuth token using the request's code", func() {
						Expect(fakeProviderB.ExchangeCallCount()).To(Equal(1))
						_, code := fakeProviderB.ExchangeArgsForCall(0)
						Expect(code).To(Equal("some-code"))
					})

					It("constructs HTTP client with disable keep alive context", func() {
						ctx, _ := fakeProviderB.ClientArgsForCall(0)
						httpClient, ok := ctx.Value(oauth2.HTTPClient).(http.Client)
						Expect(ok).To(BeTrue())
						Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
					})

					Context("when the token is verified", func() {
						BeforeEach(func() {
							fakeProviderB.VerifyReturns(true, nil)
						})

						It("responds OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("verifies using the provider's HTTP client", func() {
							Expect(fakeProviderB.ClientCallCount()).To(Equal(1))
							_, clientToken := fakeProviderB.ClientArgsForCall(0)
							Expect(clientToken).To(Equal(token))

							Expect(fakeProviderB.VerifyCallCount()).To(Equal(1))
							_, client := fakeProviderB.VerifyArgsForCall(0)
							Expect(client).To(Equal(httpClient))
						})

						Describe("the ATC-Authorization cookie", func() {
							var cookie *http.Cookie

							JustBeforeEach(func() {
								cookies := response.Cookies()
								cookie = cookies[0]
							})

							It("set to a signed token that expires in 1 day", func() {
								Expect(cookie.Name).To(Equal(auth.CookieName))
								Expect(cookie.Expires).To(BeTemporally("~", time.Now().Add(auth.CookieAge), 5*time.Second))

								Expect(cookie.Value).To(MatchRegexp(`^Bearer .*`))

								token, err := jwt.Parse(strings.Replace(cookie.Value, "Bearer ", "", -1), keyFunc)
								Expect(err).ToNot(HaveOccurred())

								Expect(token.Claims["exp"]).To(BeNumerically("==", cookie.Expires.Unix()))
								Expect(token.Valid).To(BeTrue())
							})

							It("contains the team name and ID", func() {
								token, err := jwt.Parse(strings.Replace(cookie.Value, "Bearer ", "", -1), keyFunc)
								Expect(err).ToNot(HaveOccurred())

								Expect(token.Claims["teamName"]).To(Equal(team.Name))
								Expect(token.Claims["teamID"]).To(BeNumerically("==", team.ID))
								Expect(token.Valid).To(BeTrue())
							})
						})

						It("does not redirect", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("responds with the token and deletes oauth state cookie", func() {
							cookies := response.Cookies()
							Expect(cookies).To(HaveLen(2))

							cookie := cookies[0]
							Expect(cookie.Value).To(MatchRegexp(`^Bearer .*`))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte(cookie.Value + "\n")))

							deletedCookie := cookies[1]
							Expect(deletedCookie.Name).To(Equal(auth.OAuthStateCookie))
							Expect(deletedCookie.MaxAge).To(Equal(-1))
						})
					})

					Context("when the token is not verified", func() {
						BeforeEach(func() {
							fakeProviderB.VerifyReturns(false, nil)
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
							fakeProviderB.VerifyReturns(false, errors.New("nope"))
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

			Context("when a redirect URI is in the state", func() {
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

						fakeProviderB.ExchangeReturns(token, nil)
						fakeProviderB.ClientReturns(httpClient)
					})

					It("generated the OAuth token using the request's code", func() {
						Expect(fakeProviderB.ExchangeCallCount()).To(Equal(1))
						_, code := fakeProviderB.ExchangeArgsForCall(0)
						Expect(code).To(Equal("some-code"))
					})

					Context("when the token is verified", func() {
						BeforeEach(func() {
							fakeProviderB.VerifyReturns(true, nil)
						})

						It("redirects to the redirect uri", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("main page\n")))
						})
					})

					Context("when the token is not verified", func() {
						BeforeEach(func() {
							fakeProviderB.VerifyReturns(false, nil)
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
							fakeProviderB.VerifyReturns(false, errors.New("nope"))
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

			Context("when the team cannot be found", func() {
				BeforeEach(func() {
					fakeAuthDB.GetTeamByNameReturns(db.SavedTeam{}, false, nil)
				})

				It("returns Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})

				It("does not set a cookie", func() {
					Expect(response.Cookies()).To(BeEmpty())
				})

				It("does not set exchange the token", func() {
					Expect(fakeProviderB.ExchangeCallCount()).To(Equal(0))
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
					Expect(fakeProviderB.ExchangeCallCount()).To(Equal(0))
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
					Expect(fakeProviderB.ExchangeCallCount()).To(Equal(0))
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
					Expect(fakeProviderB.ExchangeCallCount()).To(Equal(0))
				})
			})
		})

		Context("to an unknown provider", func() {
			BeforeEach(func() {
				request.URL.Path = "/auth/bogus/callback"
			})

			It("returns Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
