package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/dgrijalva/jwt-go"
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

		client = &http.Client{
			Transport: &http.Transport{},
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
				fakeProviderB.AuthCodeURLReturns(redirectTarget.URL())
			})

			It("redirects to the auth code URL", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("sup")))
			})

			It("generates the auth code with a JWT as the state", func() {
				Expect(fakeProviderB.AuthCodeURLCallCount()).To(Equal(1))

				state, _ := fakeProviderB.AuthCodeURLArgsForCall(0)

				token, err := jwt.Parse(state, func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
					}

					return signingKey.Public(), nil
				})
				Expect(err).ToNot(HaveOccurred())

				Expect(token.Claims["exp"]).To(BeNumerically("~", time.Now().Add(time.Hour).Unix(), 5))
				Expect(token.Valid).To(BeTrue())
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
