package skycmd_test

import (
	"errors"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/skymarshal/skycmd"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("skyDisplayUserIdGenerator", func() {
	var displayUserIdGenerator atc.DisplayUserIdGenerator

	Context("NewSkyDisplayUserIdGenerator", func() {
		Context("when connector is invalid", func() {
			It("should return an error", func() {
				var err error
				displayUserIdGenerator, err = skycmd.NewSkyDisplayUserIdGenerator(map[string]string{"dummy": "user"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(errors.New("invalid connector: dummy")))
				Expect(displayUserIdGenerator).To(BeNil())
			})
		})

		Context("when connector field is invalid", func() {
			It("should return an error", func() {
				var err error
				displayUserIdGenerator, err = skycmd.NewSkyDisplayUserIdGenerator(map[string]string{"ldap": "user"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(errors.New("invalid user field user of connector ldap")))
				Expect(displayUserIdGenerator).To(BeNil())
			})
		})

		Context("when configuration is valid", func() {
			BeforeEach(func() {
				var err error
				displayUserIdGenerator, err = skycmd.NewSkyDisplayUserIdGenerator(map[string]string{
					"ldap":            "user_id",
					"github":          "username",
					"bitbucket-cloud": "name",
					"cloudfoundry":    "email",
					"gitlab":          "email",
					"microsoft":       "email",
					"oauth":           "email",
					"oidc":            "email",
					"saml":            "email",
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(displayUserIdGenerator).ToNot(BeNil())
			})

			DescribeTable("DisplayUserId",
				func(connector string, expected string) {
					result := displayUserIdGenerator.DisplayUserId(connector, "userid", "username", "preferredUsername", "email")
					Expect(expected).Should(Equal(result))
				},

				Entry("ldap connector", "ldap", "userid"),
				Entry("github connector", "github", "preferredUsername"),
				Entry("bitbucket-cloud connector", "bitbucket-cloud", "username"),
				Entry("cloudfoundry connector", "cloudfoundry", "email"),
				Entry("gitlab connector", "gitlab", "email"),
				Entry("microsoft connector", "microsoft", "email"),
				Entry("oauth connector", "oauth", "email"),
				Entry("oidc connector", "oidc", "email"),
				Entry("saml connector", "saml", "email"),
			)
		})
	})
})
