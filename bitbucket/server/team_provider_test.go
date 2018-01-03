package server_test

import (
	"github.com/concourse/skymarshal/bitbucket/server"
	"github.com/concourse/skymarshal/provider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bitbucket Server Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *server.AuthConfig
		)
		BeforeEach(func() {
			authConfig = &server.AuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "Bitbucket Server",
				AuthURL:     "http://bum-bum-bum.com/oauth/v1/bitbucket-server?team_name=dudududum",
			}))
		})
	})
})
