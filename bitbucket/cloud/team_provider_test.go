package cloud_test

import (
	"github.com/concourse/skymarshal/bitbucket/cloud"
	"github.com/concourse/skymarshal/provider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bitbucket Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *cloud.AuthConfig
		)
		BeforeEach(func() {
			authConfig = &cloud.AuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "Bitbucket Cloud",
				AuthURL:     "http://bum-bum-bum.com/auth/bitbucket-cloud?team_name=dudududum",
			}))
		})
	})
})
