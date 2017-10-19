package bitbucketserver_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/bitbucketserver"
)

var _ = Describe("Bitbucket Server Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod atc.AuthMethod
			authConfig *bitbucketserver.BitbucketAuthConfig
		)
		BeforeEach(func() {
			authConfig = &bitbucketserver.BitbucketAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(atc.AuthMethod{
				Type: atc.AuthTypeOAuth,
				DisplayName: "Bitbucket Server",
				AuthURL: "http://bum-bum-bum.com/auth/bitbucket-server?team_name=dudududum",
			}))
		})
	})
})