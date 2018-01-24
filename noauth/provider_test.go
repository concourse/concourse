package noauth_test

import (
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/skymarshal/noauth"
	"github.com/concourse/skymarshal/provider"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("No Auth Provider", func() {
	Describe("TeamProvider", func() {
		var (
			found        bool
			authConfig   *noauth.NoAuthConfig
			provider     provider.Provider
			teamProvider noauth.NoAuthTeamProvider
		)

		Context("NoAuth is true", func() {
			It("verifies", func() {
				teamProvider = noauth.NoAuthTeamProvider{}
				authConfig = &noauth.NoAuthConfig{true}

				provider, found = teamProvider.ProviderConstructor(authConfig)
				Expect(found).To(BeTrue())

				verifyResult, err := provider.Verify(lagertest.NewTestLogger("test"), nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(verifyResult).To(BeTrue())
			})
		})

		Context("NoAuth is false", func() {
			It("does not verify", func() {
				teamProvider = noauth.NoAuthTeamProvider{}
				authConfig = &noauth.NoAuthConfig{false}

				provider, found = teamProvider.ProviderConstructor(authConfig)
				Expect(found).To(BeTrue())

				verifyResult, err := provider.Verify(lagertest.NewTestLogger("test"), nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(verifyResult).To(BeFalse())
			})
		})
	})

	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *noauth.NoAuthConfig
		)

		BeforeEach(func() {
			authConfig = &noauth.NoAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeNone,
				DisplayName: "No Auth",
				AuthURL:     "http://bum-bum-bum.com/auth/basic/token?team_name=dudududum",
			}))
		})
	})
})
