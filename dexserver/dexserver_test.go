package dexserver_test

import (
	"sort"

	"github.com/coreos/dex/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/skymarshal/dexserver"
	"github.com/concourse/skymarshal/skycmd"
)

var _ = Describe("Dex Server", func() {
	var config *dexserver.DexConfig
	var serverConfig server.Config

	Describe("Configuration", func() {
		BeforeEach(func() {
			config = &dexserver.DexConfig{}
		})

		JustBeforeEach(func() {
			serverConfig = dexserver.NewDexServerConfig(config)
		})

		Context("static configuration", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					IssuerURL: "http://example.com/",
				}
			})
			It("configures expected values", func() {
				Expect(serverConfig.PasswordConnector).To(Equal("local"))
				Expect(serverConfig.SupportedResponseTypes).To(ConsistOf("code", "token", "id_token"))
				Expect(serverConfig.SkipApprovalScreen).To(BeTrue())
				Expect(serverConfig.Issuer).To(Equal(config.IssuerURL))
			})
		})

		Context("when github clientId and clientSecret are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					IssuerURL: "http://example.com/",
					Flags: skycmd.AuthFlags{
						Github: skycmd.GithubFlags{
							ClientID:     "client-id",
							ClientSecret: "client-secret",
						},
					},
				}
			})

			It("should configure github connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("github"))
				Expect(connectors[0].Type).To(Equal("github"))
				Expect(connectors[0].Name).To(Equal("GitHub"))
				Expect(connectors[0].Config).To(MatchJSON(`{
					"clientID":     "client-id",
					"clientSecret": "client-secret",
					"redirectURI":  "http://example.com/callback",
					"org":          "",
					"orgs":         null,
					"hostName":     "",
					"rootCA":       ""
				}`))
			})
		})

		Context("when cf clientId, clientSecret, apiUrl, rootsCAs and skip ssl validation are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					IssuerURL: "http://example.com/",
					Flags: skycmd.AuthFlags{
						CF: skycmd.CFFlags{
							ClientID:           "client-id",
							ClientSecret:       "client-secret",
							APIURL:             "http://example.com/api",
							RootCAs:            []string{"some-ca-cert"},
							InsecureSkipVerify: false,
						},
					},
				}
			})

			It("should configure cf connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("cf"))
				Expect(connectors[0].Type).To(Equal("cf"))
				Expect(connectors[0].Name).To(Equal("Cloud Foundry"))
				Expect(connectors[0].Config).To(MatchJSON(`{
					"clientID":           "client-id",
					"clientSecret":       "client-secret",
					"redirectURI":        "http://example.com/callback",
					"apiURL":             "http://example.com/api",
					"rootCAs":            ["some-ca-cert"],
					"insecureSkipVerify": false
				}`))
			})
		})

		Context("when ldap host, user search basedn and username are configured", func() {
			BeforeEach(func() {
				ldapflags := skycmd.LDAPFlags{
					Host:               "some.host.name",
					BindDN:             "some-bind-dn",
					BindPW:             "some-bind-pw",
					InsecureNoSSL:      true,
					InsecureSkipVerify: true,
					StartTLS:           true,
					RootCA:             "/tmp/some-ca",
				}
				ldapflags.UserSearch.BaseDN = "some-user-base-dn"
				ldapflags.UserSearch.Filter = "(some-user-filter)"
				ldapflags.UserSearch.Username = "some-username"
				ldapflags.UserSearch.Scope = "some-user-scope"
				ldapflags.UserSearch.IDAttr = "some-id-attr"
				ldapflags.UserSearch.EmailAttr = "some-email-attr"
				ldapflags.UserSearch.NameAttr = "some-name-attr"
				ldapflags.GroupSearch.BaseDN = "some-group-base-dn"
				ldapflags.GroupSearch.Filter = "(some-group-filter)"
				ldapflags.GroupSearch.Scope = "some-group-scope"
				ldapflags.GroupSearch.UserAttr = "some-group-user-attr"
				ldapflags.GroupSearch.GroupAttr = "some-group-group-attr"
				ldapflags.GroupSearch.NameAttr = "some-group-name-attr"

				config = &dexserver.DexConfig{
					IssuerURL: "http://example.com/",
					Flags: skycmd.AuthFlags{
						LDAP: ldapflags,
					},
				}
			})

			It("should configure ldap connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("ldap"))
				Expect(connectors[0].Type).To(Equal("ldap"))
				Expect(connectors[0].Name).To(Equal("LDAP"))
				Expect(connectors[0].Config).To(MatchJSON(`{
					"host":               "some.host.name",
					"insecureNoSSL":      true,
					"insecureSkipVerify": true,
					"startTLS":           true,
					"rootCA":             "/tmp/some-ca",
					"rootCAData":         null,
					"bindDN":             "some-bind-dn",
					"bindPW":             "some-bind-pw",
					"usernamePrompt":     "",
					"userSearch": {
						"baseDN":    "some-user-base-dn",
						"username":  "some-username",
						"filter":    "(some-user-filter)",
						"scope":     "some-user-scope",
						"idAttr":    "some-id-attr",
						"emailAttr": "some-email-attr",
						"nameAttr":  "some-name-attr"
					},
					"groupSearch": {
						"baseDN":    "some-group-base-dn",
						"filter":    "(some-group-filter)",
						"scope":     "some-group-scope",
						"userAttr":  "some-group-user-attr",
						"groupAttr": "some-group-group-attr",
						"nameAttr":  "some-group-name-attr"
					}
				}`))
			})
		})

		Context("when local users are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					Flags: skycmd.AuthFlags{
						LocalUsers: map[string]string{
							"some-user-0": "some-password-0",
							"some-user-1": "some-password-1",
						},
					},
				}
			})

			It("should configure local connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("local"))
				Expect(connectors[0].Type).To(Equal("local"))
				Expect(connectors[0].Name).To(Equal("Username/Password"))
			})

			It("should configure local users", func() {
				passwords, err := serverConfig.Storage.ListPasswords()
				Expect(err).NotTo(HaveOccurred())

				// we're adding users from a map, which is unordered
				sort.Slice(passwords, func(i, j int) bool {
					return passwords[i].Username < passwords[j].Username
				})

				Expect(passwords[0].UserID).To(Equal("some-user-0"))
				Expect(passwords[0].Username).To(Equal("some-user-0"))
				Expect(passwords[0].Email).To(Equal("some-user-0"))
				Expect(bcrypt.CompareHashAndPassword(passwords[0].Hash, []byte("some-password-0"))).NotTo(HaveOccurred())

				Expect(passwords[1].UserID).To(Equal("some-user-1"))
				Expect(passwords[1].Username).To(Equal("some-user-1"))
				Expect(passwords[1].Email).To(Equal("some-user-1"))
				Expect(bcrypt.CompareHashAndPassword(passwords[1].Hash, []byte("some-password-1"))).NotTo(HaveOccurred())
			})
		})

		Context("when clientId and clientSecret are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					ClientID:     "some-client-id",
					ClientSecret: "some-client-secret",
					RedirectURL:  "http://example.com",
				}
			})

			It("should contain the configured clients", func() {
				clients, err := serverConfig.Storage.ListClients()
				Expect(err).NotTo(HaveOccurred())
				Expect(clients).To(HaveLen(1))
				Expect(clients[0].ID).To(Equal("some-client-id"))
				Expect(clients[0].Secret).To(Equal("some-client-secret"))
				Expect(clients[0].RedirectURIs).To(ContainElement("http://example.com"))
				Expect(clients[0].Public).To(BeFalse())
			})
		})
	})
})
