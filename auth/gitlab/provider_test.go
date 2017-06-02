package gitlab_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/gitlab"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitLab Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod atc.AuthMethod
			authConfig *gitlab.GitLabAuthConfig
		)
		BeforeEach(func() {
			authConfig = &gitlab.GitLabAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(atc.AuthMethod{
				Type:        atc.AuthTypeOAuth,
				DisplayName: "GitLab",
				AuthURL:     "http://bum-bum-bum.com/auth/gitlab?team_name=dudududum",
			}))
		})
	})

})
