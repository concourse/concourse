package github_test

import (
	"github.com/concourse/skymarshal/github"
	"github.com/concourse/skymarshal/provider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitHub Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod provider.AuthMethod
			authConfig *github.GitHubAuthConfig
		)
		BeforeEach(func() {
			authConfig = &github.GitHubAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "GitHub",
				AuthURL:     "http://bum-bum-bum.com/auth/github?team_name=dudududum",
			}))
		})
	})

})
