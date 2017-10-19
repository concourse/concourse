package bitbucket_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/bitbucket"
)

var _ = Describe("Bitbucket Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod atc.AuthMethod
			authConfig *bitbucket.BitbucketAuthConfig
		)
		BeforeEach(func() {
			authConfig = &bitbucket.BitbucketAuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(atc.AuthMethod{
				Type: atc.AuthTypeOAuth,
				DisplayName: "Bitbucket",
				AuthURL: "http://bum-bum-bum.com/auth/bitbucket?team_name=dudududum",
			}))
		})
	})
})