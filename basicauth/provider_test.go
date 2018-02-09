package basicauth_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/skymarshal/basicauth"
	"github.com/concourse/skymarshal/provider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Basic Auth Provider", func() {
	Describe("TeamProvider", func() {
		var (
			found        bool
			authConfig   *basicauth.BasicAuthConfig
			provider     provider.Provider
			teamProvider basicauth.BasicAuthTeamProvider
		)

		It("statically verifies username and passowrd", func() {
			teamProvider = basicauth.BasicAuthTeamProvider{}

			encrypted, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
			authConfig = &basicauth.BasicAuthConfig{"username", string(encrypted)}

			provider, found = teamProvider.ProviderConstructor(authConfig, "username", "password")
			Expect(found).To(BeTrue())

			verifyResult, err := provider.Verify(lagertest.NewTestLogger("test"), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(verifyResult).To(BeTrue())
		})

		It("fails if usernames don't match", func() {
			teamProvider = basicauth.BasicAuthTeamProvider{}
			authConfig = &basicauth.BasicAuthConfig{"username", "password"}

			provider, found = teamProvider.ProviderConstructor(authConfig, "not-username", "password")
			Expect(found).To(BeTrue())

			verifyResult, err := provider.Verify(lagertest.NewTestLogger("test"), nil)
			Expect(err).To(HaveOccurred())
			Expect(verifyResult).To(BeFalse())
		})

		It("fails if passwords don't match", func() {
			teamProvider = basicauth.BasicAuthTeamProvider{}
			authConfig = &basicauth.BasicAuthConfig{"username", "password"}

			provider, found = teamProvider.ProviderConstructor(authConfig, "username", "not-password")
			Expect(found).To(BeTrue())

			verifyResult, err := provider.Verify(lagertest.NewTestLogger("test"), nil)
			Expect(err).To(HaveOccurred())
			Expect(verifyResult).To(BeFalse())
		})

		It("fails to create provider if auth config is not provided", func() {
			provider, found = teamProvider.ProviderConstructor(nil, "username", "password")
			Expect(found).To(BeFalse())
		})

	})

	Describe("Config", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *basicauth.BasicAuthConfig
		)

		Context("AuthMethod", func() {
			BeforeEach(func() {
				authConfig = &basicauth.BasicAuthConfig{}
				authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
			})

			It("auth method creates path for route", func() {
				Expect(authMethod).To(Equal(provider.AuthMethod{
					Type:        provider.AuthTypeBasic,
					DisplayName: "Basic Auth",
					AuthURL:     "http://bum-bum-bum.com/auth/basic/token?team_name=dudududum",
				}))
			})
		})

		Context("Finalize", func() {
			It("won't double hash passwords if finalize is called multiple times", func() {
				authConfig = &basicauth.BasicAuthConfig{"username", "password"}

				err := authConfig.Finalize()
				Expect(err).NotTo(HaveOccurred())

				err = bcrypt.CompareHashAndPassword([]byte(authConfig.Password), []byte("password"))
				Expect(err).NotTo(HaveOccurred())

				err = authConfig.Finalize()
				Expect(err).NotTo(HaveOccurred())

				err = bcrypt.CompareHashAndPassword([]byte(authConfig.Password), []byte("password"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
