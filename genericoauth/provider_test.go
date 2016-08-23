package genericoauth_test

import (
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"

	"golang.org/x/oauth2"

	"github.com/concourse/atc/auth/genericoauth"
	"github.com/concourse/atc/auth/provider"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Generic OAuth Provider", func() {
	var (
		dbGenericOAuth *db.GenericOAuth
		redirectURI    string
		goaProvider    provider.Provider
		state          string
	)

	JustBeforeEach(func() {
		goaProvider = genericoauth.NewProvider(dbGenericOAuth, redirectURI)
	})

	BeforeEach(func() {
		dbGenericOAuth = &db.GenericOAuth{}
		redirectURI = "redirect-uri"
		state = "some-random-guid"
	})

	It("constructs HTTP client with disable keep alive context", func() {
		httpClient, err := goaProvider.PreTokenClient()
		Expect(httpClient).NotTo(BeNil())
		Expect(httpClient.Transport).NotTo(BeNil())
		Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
		Expect(err).NotTo(HaveOccurred())
	})

	It("constructs the Auth URL with the redirect uri", func() {
		authURI := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

		Expect(authURI).To(ContainSubstring("redirect_uri=redirect-uri"))
	})

	It("constructs the Auth URL with the state param", func() {
		authURI := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

		Expect(authURI).To(ContainSubstring("state=some-random-guid"))
	})

	It("doesn't do any user authorization", func() {
		verifyResult, err := goaProvider.Verify(lagertest.NewTestLogger("test"), nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(verifyResult).To(Equal(true))
	})

	Context("Auth URL params are configured", func() {
		BeforeEach(func() {
			redirectURI = "redirect-uri"
			dbGenericOAuth = &db.GenericOAuth{
				AuthURLParams: map[string]string{"param1": "value1", "param2": "value2"},
			}
		})

		It("constructs the Auth URL with the configured Auth URL params", func() {
			authURI := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

			Expect(authURI).To(ContainSubstring("param1=value1"))
			Expect(authURI).To(ContainSubstring("param2=value2"))
		})

		It("merges the passed in Auth URL params with the configured Auth URL params", func() {
			authURI := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("param3", "value3")}...)

			Expect(authURI).To(ContainSubstring("param1=value1"))
			Expect(authURI).To(ContainSubstring("param2=value2"))
			Expect(authURI).To(ContainSubstring("param3=value3"))
		})

		It("URL encodes the Auth URL params", func() {
			authURI := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("question#1", "are your tests passing?")}...)

			Expect(authURI).To(ContainSubstring("question%231=are+your+tests+passing%3F"))
		})

	})
})
