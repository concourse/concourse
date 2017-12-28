package gitlab_test

import (
	"github.com/concourse/skymarshal/gitlab"
	"github.com/concourse/skymarshal/provider"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitLab Provider", func() {
	var (
		authMethod provider.AuthMethod
		authConfig *gitlab.GitLabAuthConfig
	)

	BeforeEach(func() {
		authConfig = &gitlab.GitLabAuthConfig{}
	})

	Describe("AuthMethod", func() {
		BeforeEach(func() {
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates path for route", func() {
			Expect(authMethod).To(Equal(provider.AuthMethod{
				Type:        provider.AuthTypeOAuth,
				DisplayName: "GitLab",
				AuthURL:     "http://bum-bum-bum.com/auth/gitlab?team_name=dudududum",
			}))
		})
	})

	Describe("Validate", func() {
		BeforeEach(func() {
			authConfig.ClientID = "foo"
			authConfig.ClientSecret = "bar"
			authConfig.Groups = []string{"group1", "group2"}
		})

		Context("when client id/secret and groups are specified", func() {
			It("succeeds", func() {
				err := authConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when client id is not specified", func() {
			It("returns an error", func() {
				authConfig.ClientID = ""
				err := authConfig.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth.")))
			})
		})

		Context("when client secret is not specified", func() {
			It("returns an error", func() {
				authConfig.ClientSecret = ""
				err := authConfig.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("must specify --gitlab-auth-client-id and --gitlab-auth-client-secret to use GitLab OAuth.")))
			})
		})

		Context("when group is not specified", func() {
			It("returns an error", func() {
				authConfig.Groups = nil
				err := authConfig.Validate()
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("the following is required for gitlab-auth: groups")))
			})
		})

		Context("when client id/secret and groups specified", func() {
			It("succeeds", func() {
				err := authConfig.Validate()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

})
