package cloud_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/auth/bitbucket/cloud"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bitbucket Provider", func() {
	Describe("AuthMethod", func() {
		var (
			authMethod atc.AuthMethod
			authConfig *cloud.AuthConfig
		)
		BeforeEach(func() {
			authConfig = &cloud.AuthConfig{}
			authMethod = authConfig.AuthMethod("http://bum-bum-bum.com", "dudududum")
		})

		It("creates a path for route", func() {
			Expect(authMethod).To(Equal(atc.AuthMethod{
				Type:        atc.AuthTypeOAuth,
				DisplayName: "Bitbucket Cloud",
				AuthURL:     "http://bum-bum-bum.com/auth/bitbucket-cloud?team_name=dudududum",
			}))
		})
	})
})
