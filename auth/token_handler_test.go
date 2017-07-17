package auth_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/authfakes"
	"github.com/concourse/atc/auth/provider/providerfakes"
	"github.com/concourse/atc/db/dbfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

var _ = Describe("TokenHandler", func() {
	var (
		fakeProvider *providerfakes.FakeProvider

		fakeProviderFactory *authfakes.FakeProviderFactory

		fakeTeamFactory *dbfakes.FakeTeamFactory
		fakeTeam        *dbfakes.FakeTeam

		signingKey *rsa.PrivateKey

		expire time.Duration

		server *httptest.Server
		client *http.Client

		request  *http.Request
		response *http.Response
	)

	BeforeEach(func() {

		fakeProvider = new(providerfakes.FakeProvider)
		fakeProviderFactory = new(authfakes.FakeProviderFactory)

		fakeTeam = new(dbfakes.FakeTeam)
		fakeTeamFactory = new(dbfakes.FakeTeamFactory)

		var err error
		signingKey, err = rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		expire = 24 * time.Hour

		handler, err := auth.NewOAuthHandler(
			lagertest.NewTestLogger("test"),
			fakeProviderFactory,
			fakeTeamFactory,
			signingKey,
			expire,
			false,
		)

		Expect(err).ToNot(HaveOccurred())

		server = httptest.NewServer(handler)

		fakeProviderFactory.GetProviderReturns(fakeProvider, true, nil)

		request, err = http.NewRequest("POST", server.URL, strings.NewReader("some-token"))

		Expect(err).NotTo(HaveOccurred())

		request.URL.Path = "/auth/some-provider/token"

		request.URL.RawQuery = url.Values{
			"team_name": {"some-team"},
		}.Encode()

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	JustBeforeEach(func() {
		var err error

		response, err = client.Do(request)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("POST /auth/some-provider/token", func() {

		Context("A token is present", func() {
			BeforeEach(func() {
				fakeTeam.NameReturns("some-team")
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{BasicAuthUsername: "some-username"})
				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("and the user is verified", func() {

				BeforeEach(func() {
					fakeProvider.VerifyReturns(true, nil)
				})

				It("returns a valid token", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
					Expect(ioutil.ReadAll(response.Body)).To(HavePrefix("Bearer "))
				})
			})

			Context("and the user is not verified", func() {

				BeforeEach(func() {
					fakeProvider.VerifyReturns(false, nil)
				})

				It("returns a correct error response", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					Expect(ioutil.ReadAll(response.Body)).ToNot(HavePrefix("Bearer "))
				})
			})

			Context("and the user can not be verified", func() {

				BeforeEach(func() {
					fakeProvider.VerifyReturns(false, errors.New("Exception"))
				})

				It("returns a correct error response", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("and the team can not be found", func() {

				BeforeEach(func() {
					fakeProvider.VerifyReturns(true, nil)
					fakeTeamFactory.FindTeamReturns(nil, false, nil)
				})

				It("returns a correct error response", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("and the team can not be found due to an error", func() {

				BeforeEach(func() {
					fakeProvider.VerifyReturns(true, nil)
					fakeTeamFactory.FindTeamReturns(nil, false, errors.New("Exception"))
				})

				It("returns a correct error response", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("A token is not present", func() {
			BeforeEach(func() {
				fakeTeam.NameReturns("some-team")
				fakeTeam.BasicAuthReturns(&atc.BasicAuth{BasicAuthUsername: "some-username"})
				fakeTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				request.Body = nil
				request.ContentLength = 0
			})

			It("returns a correct error response", func() {
				Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})
})
