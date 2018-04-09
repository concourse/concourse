package dexserver_test

import (
	"sort"

	"github.com/coreos/dex/server"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"

	"github.com/concourse/skymarshal/dexserver"
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

		Context("when github clientId and clientSecret are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					IssuerURL:          "http://example.com/",
					GithubClientID:     "client-id",
					GithubClientSecret: "client-secret",
				}
			})

			It("should configure github connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("github"))
				Expect(connectors[0].Type).To(Equal("github"))
				Expect(connectors[0].Name).To(Equal("Github"))
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

		Context("when local users are configured", func() {
			BeforeEach(func() {
				config = &dexserver.DexConfig{
					LocalUsers: map[string]string{
						"some-user-0": "some-password-0",
						"some-user-1": "some-password-1",
					},
				}
			})

			It("should configure local connector", func() {
				connectors, err := serverConfig.Storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())

				Expect(connectors[0].ID).To(Equal("local"))
				Expect(connectors[0].Type).To(Equal("local"))
				Expect(connectors[0].Name).To(Equal("Username"))
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
