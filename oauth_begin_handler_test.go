package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/fakes"
)

var _ = Describe("OAuthBeginHandler", func() {
	var (
		fakeProviderA *fakes.FakeProvider
		fakeProviderB *fakes.FakeProvider

		signingKey *rsa.PrivateKey

		cookieJar *cookiejar.Jar

		server *httptest.Server
		client *http.Client
	)

	BeforeEach(func() {
		fakeProviderA = new(fakes.FakeProvider)
		fakeProviderB = new(fakes.FakeProvider)

		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())

		handler, err := auth.NewOAuthHandler(
			lagertest.NewTestLogger("test"),
			auth.Providers{
				"a": fakeProviderA,
				"b": fakeProviderB,
			},
			signingKey,
		)
		Expect(err).ToNot(HaveOccurred())

		server = httptest.NewServer(handler)

		cookieJar, err = cookiejar.New(nil)
		Expect(err).ToNot(HaveOccurred())

		client = &http.Client{
			Transport: &http.Transport{},
			Jar:       cookieJar,
		}
	})

	Describe("GET /auth/:provider", func() {
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
				request.URL.Path = "/auth/b"
				request.URL.RawQuery = url.Values{
					"redirect": {"/some-path"},
				}.Encode()

				fakeProviderB.AuthCodeURLReturns(redirectTarget.URL())
			})

			It("redirects to the auth code URL", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("sup")))
			})

			It("generates the auth code with a base64-encoded redirect URI as the state", func() {
				Expect(fakeProviderB.AuthCodeURLCallCount()).To(Equal(1))

				state, _ := fakeProviderB.AuthCodeURLArgsForCall(0)

				decoded, err := base64.RawURLEncoding.DecodeString(state)
				Expect(err).ToNot(HaveOccurred())

				var oauthState auth.OAuthState
				err = json.Unmarshal(decoded, &oauthState)
				Expect(err).ToNot(HaveOccurred())

				Expect(oauthState.Redirect).To(Equal("/some-path"))
			})

			It("sets the base64-encoded redirect URI as the OAuth state cookie", func() {
				Expect(fakeProviderB.AuthCodeURLCallCount()).To(Equal(1))

				state, _ := fakeProviderB.AuthCodeURLArgsForCall(0)

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

			It("returns Not Found", func() {
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
