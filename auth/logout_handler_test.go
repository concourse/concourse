package auth_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/skymarshal/auth"
	"github.com/concourse/skymarshal/auth/authfakes"
)

var _ = Describe("LogOutHandler", func() {
	Describe("GET /auth/logout", func() {
		var (
			fakeProviderFactory    *authfakes.FakeProviderFactory
			fakeCSRFTokenGenerator *authfakes.FakeCSRFTokenGenerator
			fakeAuthTokenGenerator *authfakes.FakeAuthTokenGenerator
			server                 *httptest.Server
			client                 *http.Client
			request                *http.Request
			response               *http.Response
			err                    error
			expire                 time.Duration
		)

		BeforeEach(func() {
			fakeTeamFactory := new(dbfakes.FakeTeamFactory)
			fakeProviderFactory = new(authfakes.FakeProviderFactory)
			fakeCSRFTokenGenerator = new(authfakes.FakeCSRFTokenGenerator)
			fakeAuthTokenGenerator = new(authfakes.FakeAuthTokenGenerator)
			Expect(err).ToNot(HaveOccurred())
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

			mux := http.NewServeMux()
			mux.Handle("/auth/", handler)

			server = httptest.NewServer(mux)

			client = &http.Client{
				Transport: &http.Transport{},
			}

			request, err = http.NewRequest("GET", server.URL+"/auth/logout", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes ATC-Authorization cookie", func() {
			cookies := response.Cookies()
			Expect(len(cookies)).To(Equal(1))

			deletedCookie := cookies[0]
			Expect(deletedCookie.Name).To(Equal(auth.AuthCookieName))
			Expect(deletedCookie.MaxAge).To(Equal(-1))
		})
	})
})
