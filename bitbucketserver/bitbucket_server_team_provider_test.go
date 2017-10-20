package bitbucketserver_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/bitbucketserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bitbucket Server Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod atc.AuthMethod
			authConfig *bitbucketserver.BitbucketServerAuthConfig
		)
		BeforeEach(func() {
			authConfig = &bitbucketserver.BitbucketServerAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(atc.AuthMethod{
				Type:        atc.AuthTypeOAuth,
				DisplayName: "Bitbucket Server",
				AuthURL:     "http://bum-bum-bum.com/oauth/v1/bitbucket-server?team_name=dudududum",
			}))
		})
	})
})
