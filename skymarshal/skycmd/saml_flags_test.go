package skycmd_test

import (
	"os"

	"github.com/concourse/concourse/skymarshal/skycmd"
	flags "github.com/jessevdk/go-flags"
	"github.com/vito/twentythousandtonnesofcrudeoil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SAMLTeamFlags", func() {
	var (
		cmd struct {
			SAML skycmd.SAMLTeamFlags `group:"SAML" namespace:"main-team-saml"`
		}
		parser *flags.Parser
	)

	BeforeEach(func() {
		cmd.SAML = skycmd.SAMLTeamFlags{}
		parser = flags.NewParser(&cmd, flags.None)
		parser.NamespaceDelimiter = "-"
		twentythousandtonnesofcrudeoil.TheEnvironmentIsPerfectlySafe(parser, "CONCOURSE_")
	})

	AfterEach(func() {
		os.Unsetenv("CONCOURSE_MAIN_TEAM_SAML_GROUP")
	})

	parseGroupsFromEnv := func(value string) []string {
		os.Setenv("CONCOURSE_MAIN_TEAM_SAML_GROUP", value)
		_, err := parser.ParseArgs(nil)
		Expect(err).ToNot(HaveOccurred())
		return cmd.SAML.GetGroups()
	}

	Describe("groups from environment variables", func() {
		Context("when the value contains commas", func() {
			It("splits into multiple groups, preserving existing behavior", func() {
				groups := parseGroupsFromEnv("group1,group2")
				Expect(groups).To(Equal([]string{"group1", "group2"}))
			})
		})

		Context("when a group is wrapped in double quotes", func() {
			It("preserves commas within the quoted group", func() {
				groups := parseGroupsFromEnv(`"CN=my_concourse_admin,OU=SecurityGroups,DC=example,DC=com"`)
				Expect(groups).To(Equal([]string{"CN=my_concourse_admin,OU=SecurityGroups,DC=example,DC=com"}))
			})

			It("allows mixing quoted and unquoted groups", func() {
				groups := parseGroupsFromEnv(`"CN=admins,DC=example,DC=com",developers`)
				Expect(groups).To(Equal([]string{"CN=admins,DC=example,DC=com", "developers"}))
			})

			It("allows multiple quoted groups", func() {
				groups := parseGroupsFromEnv(`"CN=a,DC=com","CN=b,DC=com"`)
				Expect(groups).To(Equal([]string{"CN=a,DC=com", "CN=b,DC=com"}))
			})

			It("strips quotes from a quoted group without commas", func() {
				groups := parseGroupsFromEnv(`"developers"`)
				Expect(groups).To(Equal([]string{"developers"}))
			})
		})

		Context("when a quote is never closed", func() {
			It("leaves the values untouched", func() {
				groups := parseGroupsFromEnv(`"group1,group2`)
				Expect(groups).To(Equal([]string{`"group1`, "group2"}))
			})
		})
	})

	Describe("groups from flags", func() {
		It("keeps commas without requiring quotes", func() {
			_, err := parser.ParseArgs([]string{
				"--main-team-saml-group", "CN=admins,OU=Groups,DC=example,DC=com",
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(cmd.SAML.GetGroups()).To(Equal([]string{"CN=admins,OU=Groups,DC=example,DC=com"}))
		})
	})
})
