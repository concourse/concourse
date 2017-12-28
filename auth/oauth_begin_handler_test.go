package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/auth/authfakes"
	"github.com/concourse/skymarshal/provider/providerfakes"
)

var _ = Describe("OAuthBeginHandler", func() {
	var (
		fakeProvider *providerfakes.FakeProvider

		fakeProviderFactory    *authfakes.FakeProviderFactory
		fakeCSRFTokenGenerator *authfakes.FakeCSRFTokenGenerator
		fakeAuthTokenGenerator *authfakes.FakeAuthTokenGenerator

		fakeTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam        *dbfakes.FakeTeam

		expire time.Duration

		cookieJar *cookiejar.Jar

		server *httptest.Server
		client *http.Client
	)

	BeforeEach(func() {
		fakeProvider = new(providerfakes.FakeProvider)

		fakeProviderFactory = new(authfakes.FakeProviderFactory)
		fakeCSRFTokenGenerator = new(authfakes.FakeCSRFTokenGenerator)
		fakeAuthTokenGenerator = new(authfakes.FakeAuthTokenGenerator)

		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)

		expire = 24 * time.Hour

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

		server = httptest.NewServer(handler)

		cookieJar, err = cookiejar.New(nil)
		Expect(err).ToNot(HaveOccurred())

		client = &http.Client{
			Transport: &http.Transport{},
			Jar:       cookieJar,
		}

		fakeProviderFactory.GetProviderReturns(fakeProvider, true, nil)
	})

	Describe("GET /auth/:provider/teams/:team_name", func() {
		var redirectTarget *ghttp.Server
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			redirectTarget = ghttp.NewServer()
			redirectTarget.RouteToHandler("GET", "/", ghttp.RespondWith(http.StatusOK, "sup"))

			var err error

			request, err = http.NewRequest("GET", server.URL, nil)
			Expect(err).NotTo(HaveOccurred())

			request.URL.RawQuery = url.Values{
				"redirect":  {"/some-path"},
				"team_name": {"some-team"},
			}.Encode()
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the team exists", func() {
			BeforeEach(func() {
				fakeTeam.NameReturns("some-team")
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{BasicAuthUsername: "some-username"})
				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("to a known provider", func() {
				BeforeEach(func() {
					request.URL.Path = "/auth/provider-name"
					fakeProvider.AuthCodeURLReturns(redirectTarget.URL(), nil)
				})

				It("redirects to the auth code URL", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("sup")))
				})

				It("generates the auth code with a base64-encoded redirect URI and team name as the state", func() {
					Expect(fakeProvider.AuthCodeURLCallCount()).To(Equal(1))

					state, _ := fakeProvider.AuthCodeURLArgsForCall(0)

					decoded, err := base64.RawURLEncoding.DecodeString(state)
					Expect(err).ToNot(HaveOccurred())

					var oauthState auth.OAuthState
					err = json.Unmarshal(decoded, &oauthState)
					Expect(err).ToNot(HaveOccurred())
					Expect(oauthState.TeamName).To(Equal("some-team"))
					Expect(oauthState.Redirect).To(Equal("/some-path"))
				})

				It("sets the base64-encoded redirect URI as the OAuth state cookie", func() {
					Expect(fakeProvider.AuthCodeURLCallCount()).To(Equal(1))

					state, _ := fakeProvider.AuthCodeURLArgsForCall(0)

					serverURL, err := url.Parse(server.URL)
					Expect(err).ToNot(HaveOccurred())

					Expect(cookieJar.Cookies(serverURL)).To(ContainElement(&http.Cookie{
						Name:  auth.OAuthStateCookie,
						Value: state,
					}))
				})
			})

			Context("to an unknown provider", func() {
				BeforeEach(func() {
					request.URL.Path = "/auth/bogus"
				})

				It("returns 404 not found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when the team doesn't exist", func() {
			BeforeEach(func() {
				request.URL.Path = "/auth/b"

				fakeTeamFactory.FindTeamReturns(fakeTeam, false, nil)
			})

			It("returns 404 not found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		Context("when looking up the team fails", func() {
			var disaster error
			BeforeEach(func() {
				request.URL.Path = "/auth/b"

				disaster = errors.New("out of service")
				fakeTeamFactory.FindTeamReturns(fakeTeam, false, disaster)
			})

			It("returns 500 internal server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})
	})
})
