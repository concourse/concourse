package genericoauth_test

import (
	"net/http"

	"code.cloudfoundry.org/lager/lagertest"

	"golang.org/x/oauth2"

	"github.com/concourse/skymarshal/genericoauth"
	"github.com/concourse/skymarshal/provider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Generic OAuth Provider", func() {
	Describe("Provider Constructor", func() {
		var (
			authConfig          *genericoauth.GenericOAuthConfig
			redirectURI         string
			goaProvider         provider.Provider
			state               string
			found               bool
			genericTeamProvider genericoauth.GenericTeamProvider
		)

		BeforeEach(func() {
			redirectURI = "redirect-uri"
			state = "some-random-guid"
			authConfig = &genericoauth.GenericOAuthConfig{}
		})

		JustBeforeEach(func() {
			genericTeamProvider = genericoauth.GenericTeamProvider{}
			goaProvider, found = genericTeamProvider.ProviderConstructor(authConfig, redirectURI)
			Expect(found).To(BeTrue())
		})

		It("constructs HTTP client with disable keep alive context", func() {
			httpClient, err := goaProvider.PreTokenClient()
			Expect(httpClient).NotTo(BeNil())
			Expect(httpClient.Transport).NotTo(BeNil())
			Expect(httpClient.Transport.(*http.Transport).DisableKeepAlives).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("constructs the Auth URL with the redirect uri", func() {
			authURI, err := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

			Expect(authURI).To(ContainSubstring("redirect_uri=redirect-uri"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("constructs the Auth URL with the state param", func() {
			authURI, err := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

			Expect(authURI).To(ContainSubstring("state=some-random-guid"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("doesn't do any user authorization", func() {
			verifyResult, err := goaProvider.Verify(lagertest.NewTestLogger("test"), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(verifyResult).To(Equal(true))
		})

		Context("Auth URL params are configured", func() {
			BeforeEach(func() {
				redirectURI = "redirect-uri"

				authConfig = &genericoauth.GenericOAuthConfig{
					AuthURLParams: map[string]string{"param1": "value1", "param2": "value2"},
				}
			})

			It("constructs the Auth URL with the configured Auth URL params", func() {
				authURI, err := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{}...)

				Expect(authURI).To(ContainSubstring("param1=value1"))
				Expect(authURI).To(ContainSubstring("param2=value2"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("merges the passed in Auth URL params with the configured Auth URL params", func() {
				authURI, err := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("param3", "value3")}...)

				Expect(authURI).To(ContainSubstring("param1=value1"))
				Expect(authURI).To(ContainSubstring("param2=value2"))
				Expect(authURI).To(ContainSubstring("param3=value3"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("URL encodes the Auth URL params", func() {
				authURI, err := goaProvider.AuthCodeURL(state, []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("question#1", "are your tests passing?")}...)

				Expect(authURI).To(ContainSubstring("question%231=are+your+tests+passing%3F"))
				Expect(err).NotTo(HaveOccurred())
			})

		})
	})

	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *genericoauth.GenericOAuthConfig
		)
		BeforeEach(func() {
			authConfig = &genericoauth.GenericOAuthConfig{DisplayName: "duck-song"}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "duck-song",
				AuthURL:     "http://bum-bum-bum.com/auth/oauth?team_name=dudududum",
			}))
		})
	})
})
