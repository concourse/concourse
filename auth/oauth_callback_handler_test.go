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

	"code.cloudfoundry.org/lager/lagertest"
	jwt "github.com/dgrijalva/jwt-go"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"regexp"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/auth/provider/providerfakes"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
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

		fakeProviderFactory *authfakes.FakeProviderFactory

		fakeTeamDB *dbfakes.FakeTeamDB

		signingKey *rsa.PrivateKey

		expire time.Duration

		server *httptest.Server
		client *http.Client

		team db.SavedTeam
	)

	BeforeEach(func() {
		fakeProvider = new(providerfakes.FakeProvider)

		fakeProviderFactory = new(authfakes.FakeProviderFactory)

		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		expire = 24 * time.Hour

		fakeProviderFactory.GetProviderStub = func(team db.SavedTeam, providerName string) (provider.Provider, bool, error) {
			if providerName == "some-provider" {
				return fakeProvider, true, nil
			}
			return nil, false, nil
		}

		preTokenClient = &http.Client{Timeout: 31 * time.Second}
		fakeProvider.PreTokenClientReturns(preTokenClient, nil)

		team = db.SavedTeam{
			Team: db.Team{
				Name: "some-team",
			},
		}

		fakeTeamDBFactory := new(dbfakes.FakeTeamDBFactory)
		fakeTeamDB = new(dbfakes.FakeTeamDB)
		fakeTeamDB.GetTeamReturns(team, true, nil)
		fakeTeamDBFactory.GetTeamDBReturns(fakeTeamDB)

		handler, err := auth.NewOAuthHandler(
			lagertest.NewTestLogger("test"),
			fakeProviderFactory,
			fakeTeamDBFactory,
			signingKey,
			expire,
		)
		Expect(err).ToNot(HaveOccurred())

		mux := http.NewServeMux()
		mux.Handle("/auth/", handler)
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
						_, code := fakeProvider.ExchangeArgsForCall(0)
						Expect(code).To(Equal("some-code"))
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
						Expect(argTeam).To(Equal(team))
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
								cookie = cookies[0]
							})

							It("set to a signed token that expires in 1 day", func() {
								Expect(cookie.Name).To(Equal(auth.CookieName))
								Expect(cookie.Expires).To(BeTemporally("~", time.Now().Add(24*time.Hour), 5*time.Second))

								Expect(cookie.Value).To(MatchRegexp(`^Bearer .*`))

								token, err := jwt.Parse(strings.Replace(cookie.Value, "Bearer ", "", -1), keyFunc)
								Expect(err).ToNot(HaveOccurred())

								claims := token.Claims.(jwt.MapClaims)
								Expect(claims["exp"]).To(BeNumerically("==", cookie.Expires.Unix()))
								Expect(token.Valid).To(BeTrue())
							})

							It("contains the team name and ID", func() {
								token, err := jwt.Parse(strings.Replace(cookie.Value, "Bearer ", "", -1), keyFunc)
								Expect(err).ToNot(HaveOccurred())

								claims := token.Claims.(jwt.MapClaims)
								Expect(claims["teamName"]).To(Equal(team.Name))
								Expect(claims["teamID"]).To(BeNumerically("==", team.ID))
								Expect(token.Valid).To(BeTrue())
							})
						})

						It("does not redirect", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("responds with the success page and deletes oauth state cookie", func() {
							cookies := client.Jar.Cookies(request.URL)
							Expect(cookies).To(HaveLen(2))

							cookie := cookies[0]
							Expect(cookie.Value).To(MatchRegexp(`^Bearer .*`))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("fly success page\n")))

							deletedCookie := cookies[1]
							Expect(deletedCookie.Name).To(Equal(auth.OAuthStateCookie))
							Expect(deletedCookie.MaxAge).To(Equal(-1))
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
						fakeTeamDB.GetTeamReturns(db.SavedTeam{}, false, nil)
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
						_, code := fakeProvider.ExchangeArgsForCall(0)
						Expect(code).To(Equal("some-code"))
					})

					Context("when the token is verified", func() {
						BeforeEach(func() {
							fakeProvider.VerifyReturns(true, nil)
						})

						It("redirects to the redirect uri", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
							Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("main page\n")))
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
