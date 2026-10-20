package skycmd_test

import (
	"bytes"
	"encoding/json"
	"log/slog"

	"github.com/concourse/concourse/v8/skymarshal/skycmd"
	"github.com/concourse/dex/connector/ldap"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LDAPFlags", func() {
	var flags *skycmd.LDAPFlags

	BeforeEach(func() {
		flags = &skycmd.LDAPFlags{
			Host:   "ldap.example.com:636",
			BindDN: "cn=admin,dc=example,dc=com",
			BindPW: "secret",
		}
		flags.UserSearch.BaseDN = "ou=people,dc=example,dc=com"
		flags.UserSearch.Username = "uid"
		flags.GroupSearch.BaseDN = "ou=groups,dc=example,dc=com"
		flags.GroupSearch.NameAttr = "cn"
	})

	serialize := func() ldap.Config {
		payload, err := flags.Serialize("https://ci.example.com/sky/issuer/callback")
		Expect(err).ToNot(HaveOccurred())

		var config ldap.Config
		Expect(json.Unmarshal(payload, &config)).To(Succeed())
		return config
	}

	// open runs the config through dex and returns what dex logged and the
	// group matchers it ended up using.
	open := func(config ldap.Config) (string, []ldap.UserMatcher) {
		logs := new(bytes.Buffer)
		_, err := config.Open("ldap", slog.New(slog.NewTextHandler(logs, nil)))
		Expect(err).ToNot(HaveOccurred())
		return logs.String(), config.GroupSearch.UserMatchers
	}

	Describe("Serialize", func() {
		Context("when group-search-user-attr and group-search-group-attr are set", func() {
			BeforeEach(func() {
				flags.GroupSearch.UserAttr = "DN"
				flags.GroupSearch.GroupAttr = "member"
			})

			It("passes them to dex as a user matcher", func() {
				config := serialize()
				Expect(config.GroupSearch.UserMatchers).To(Equal([]ldap.UserMatcher{
					{UserAttr: "DN", GroupAttr: "member"},
				}))
			})

			It("does not use the deprecated userAttr and groupAttr fields", func() {
				config := serialize()
				Expect(config.GroupSearch.UserAttr).To(BeEmpty())
				Expect(config.GroupSearch.GroupAttr).To(BeEmpty())
			})
		})

		DescribeTable("matches groups as the deprecated fields did, without dex's deprecation warning",
			func(userAttr, groupAttr string) {
				flags.GroupSearch.UserAttr = userAttr
				flags.GroupSearch.GroupAttr = groupAttr

				legacy := serialize()
				legacy.GroupSearch.UserMatchers = nil
				legacy.GroupSearch.UserAttr = userAttr
				legacy.GroupSearch.GroupAttr = groupAttr

				legacyLogs, legacyMatchers := open(legacy)
				logs, matchers := open(serialize())

				Expect(legacyLogs).To(ContainSubstring("deprecated"))
				Expect(logs).ToNot(ContainSubstring("deprecated"))
				Expect(matchers).To(Equal(legacyMatchers))
			},
			Entry("DN and member", "DN", "member"),
			Entry("Active Directory distinguishedName", "distinguishedName", "member"),
			Entry("Active Directory nested groups", "DN", "member:1.2.840.113556.1.4.1941:"),
		)

		Context("when no user attribute is set", func() {
			BeforeEach(func() {
				flags.GroupSearch.GroupAttr = "member"
			})

			It("sends no group matching attributes and dex logs no deprecation warning", func() {
				config := serialize()
				Expect(config.GroupSearch.UserMatchers).To(BeEmpty())
				Expect(config.GroupSearch.UserAttr).To(BeEmpty())
				Expect(config.GroupSearch.GroupAttr).To(BeEmpty())

				logs, _ := open(config)
				Expect(logs).ToNot(ContainSubstring("deprecated"))
			})
		})
	})
})
