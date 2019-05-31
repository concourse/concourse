package dexserver_test

import (
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/v5/atc/atccmd"
	"github.com/concourse/concourse/v5/skymarshal/dexserver"
	"github.com/concourse/concourse/v5/skymarshal/skycmd"
	store "github.com/concourse/concourse/v5/skymarshal/storage"
	"github.com/concourse/dex/server"
	"github.com/concourse/dex/storage"
	"github.com/concourse/flag"
	"github.com/jessevdk/go-flags"
	"golang.org/x/crypto/bcrypt"
)

var _ = Describe("Dex Server", func() {
	var config *dexserver.DexConfig
	var serverConfig server.Config
	var storage storage.Storage
	var logger lager.Logger
	var err error

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("dex")

		storage, err = store.NewPostgresStorage(logger, flag.PostgresConfig{
			Host:     "127.0.0.1",
			Port:     uint16(5433 + GinkgoParallelNode()),
			User:     "postgres",
			SSLMode:  "disable",
			Database: "testdb",
		})
		Expect(err).ToNot(HaveOccurred())

		config = &dexserver.DexConfig{
			Logger:  logger,
			Storage: storage,
		}
	})

	AfterEach(func() {
		storage.Close()
	})

	JustBeforeEach(func() {
		serverConfig, err = dexserver.NewDexServerConfig(config)
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Configuration", func() {

		Context("static configuration", func() {
			BeforeEach(func() {
				config.IssuerURL = "http://example.com/"
			})

			It("configures expected values", func() {
				Expect(serverConfig.PasswordConnector).To(Equal("local"))
				Expect(serverConfig.SupportedResponseTypes).To(ConsistOf("code", "token", "id_token"))
				Expect(serverConfig.SkipApprovalScreen).To(BeTrue())
				Expect(serverConfig.Issuer).To(Equal(config.IssuerURL))
				Expect(serverConfig.Logger).NotTo(BeNil())
			})
		})

		Context("when local users are configured", func() {

			ConfiguresUsersCorrectly := func() {
				It("should configure local connector", func() {
					connectors, err := storage.ListConnectors()
					Expect(err).NotTo(HaveOccurred())

					Expect(connectors[0].ID).To(Equal("local"))
					Expect(connectors[0].Type).To(Equal("local"))
					Expect(connectors[0].Name).To(Equal("Username/Password"))
				})

				It("should configure local users", func() {
					passwords, err := storage.ListPasswords()
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
			}

			Context("when the user's password is provided as a bcrypt hash", func() {
				BeforeEach(func() {
					config.Flags = skycmd.AuthFlags{
						LocalUsers: map[string]string{
							"some-user-0": "$2a$10$3veRX245rLrpOKrgu7jIyOEKF5Km5tY86bZql6/oTMssgPO/6XJju",
							"some-user-1": "$2a$10$31qaZYMqx7mplkLoMrpPHeF3xf5eN37Zyv3e/QdPUs6S6IqrDA9Du",
						},
					}
				})

				ConfiguresUsersCorrectly()
			})

			Context("when the user's password is provided in plaintext", func() {
				BeforeEach(func() {
					config.Flags = skycmd.AuthFlags{
						LocalUsers: map[string]string{
							"some-user-0": "some-password-0",
							"some-user-1": "some-password-1",
						},
					}
				})

				ConfiguresUsersCorrectly()

				Context("when a user's password is changed", func() {
					BeforeEach(func() {
						// First create the first config based on the parent Context
						serverConfig, err = dexserver.NewDexServerConfig(config)
						Expect(err).ToNot(HaveOccurred())

						// The final config will be created in the JustBeforeEach block
						config.Flags = skycmd.AuthFlags{
							LocalUsers: map[string]string{
								"some-user-0": "some-password-0",
								"some-user-1": "some-password-1-changed",
							},
						}
					})

					It("should update the user's password", func() {
						passwords, err := storage.ListPasswords()
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
						Expect(bcrypt.CompareHashAndPassword(passwords[1].Hash, []byte("some-password-1-changed"))).NotTo(HaveOccurred())
					})
				})

				Context("when a user is then removed", func() {
					BeforeEach(func() {
						// First create the first config based on the parent Context
						serverConfig, err = dexserver.NewDexServerConfig(config)
						Expect(err).ToNot(HaveOccurred())

						// The final config will be created in the JustBeforeEach block
						config.Flags = skycmd.AuthFlags{
							LocalUsers: map[string]string{
								"some-user-0": "some-password-0",
							},
						}
					})

					It("should remove the user's password", func() {
						passwords, err := storage.ListPasswords()
						Expect(err).NotTo(HaveOccurred())

						Expect(len(passwords)).To(Equal(1))

						Expect(passwords[0].UserID).To(Equal("some-user-0"))
						Expect(passwords[0].Username).To(Equal("some-user-0"))
						Expect(passwords[0].Email).To(Equal("some-user-0"))
						Expect(bcrypt.CompareHashAndPassword(passwords[0].Hash, []byte("some-password-0"))).NotTo(HaveOccurred())
					})
				})
			})
		})

		Context("when clientId and clientSecret are configured", func() {
			BeforeEach(func() {
				config.ClientID = "some-client-id"
				config.ClientSecret = "some-client-secret"
				config.RedirectURL = "http://example.com"
			})

			It("should contain the configured clients", func() {
				clients, err := storage.ListClients()
				Expect(err).NotTo(HaveOccurred())
				Expect(clients).To(HaveLen(1))
				Expect(clients[0].ID).To(Equal("some-client-id"))
				Expect(clients[0].Secret).To(Equal("some-client-secret"))
				Expect(clients[0].RedirectURIs).To(ContainElement("http://example.com"))
				Expect(clients[0].Public).To(BeFalse())
			})
		})

		Context("when oauth provider is used", func() {
			var (
				cmd    *atccmd.RunCommand
				parser *flags.Parser
			)

			BeforeEach(func() {
				cmd = &atccmd.RunCommand{}

				parser = flags.NewParser(cmd, flags.Default^flags.PrintErrors)
				parser.NamespaceDelimiter = "-"

				args := []string{
					"--oauth-display-name=generic-provider-final",
					"--oauth-client-id=client-id",
					"--oauth-client-secret=client-secret",
					"--oauth-auth-url=https://example.com/authorize",
					"--oauth-token-url=https://example.com/token",
					"--oauth-userinfo-url=https://example.com/userinfo",
				}
				authGroup := parser.Group.Find("Authentication")
				Expect(authGroup).ToNot(BeNil())

				skycmd.WireConnectors(authGroup)
				skycmd.WireTeamConnectors(authGroup.Find("Authentication (Main Team)"))

				args, err := parser.ParseArgs(args)
				Expect(err).NotTo(HaveOccurred())

				config.IssuerURL = "http://example.com/"
				config.Flags = cmd.Auth.AuthFlags
			})

			It("sets up an oauth connector", func() {
				connectors, err := storage.ListConnectors()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(connectors)).To(Equal(1))

				Expect(connectors[0].Name).To(Equal("generic-provider-final"))
			})

			Context("when oauth params are changed", func() {
				BeforeEach(func() {
					// First create the first config based on the parent Context
					serverConfig, err = dexserver.NewDexServerConfig(config)
					Expect(err).ToNot(HaveOccurred())

					// The final config will be created in the JustBeforeEach block
					args := []string{
						"--oauth-display-name=generic-provider-new-name",
						"--oauth-client-id=client-id",
						"--oauth-client-secret=client-secret",
						"--oauth-auth-url=https://example.com/authorize",
						"--oauth-token-url=https://example.com/token",
						"--oauth-userinfo-url=https://example.com/userinfo",
					}

					_, err := parser.ParseArgs(args)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should update the oauth connector", func() {
					connectors, err := storage.ListConnectors()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(connectors)).To(Equal(1))

					Expect(connectors[0].Name).To(Equal("generic-provider-new-name"))
				})

			})

			Context("when oauth params are then removed", func() {
				BeforeEach(func() {
					// First create the first config based on the parent Context
					serverConfig, err = dexserver.NewDexServerConfig(config)
					Expect(err).ToNot(HaveOccurred())

					// The final config will be created in the JustBeforeEach block
					args := []string{
						"--oauth-display-name=",
						"--oauth-client-id=",
						"--oauth-client-secret=",
						"--oauth-auth-url=",
						"--oauth-token-url=",
						"--oauth-userinfo-url=",
					}

					_, err := parser.ParseArgs(args)
					Expect(err).NotTo(HaveOccurred())
				})

				It("should remove the oauth connector", func() {
					connectors, err := storage.ListConnectors()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(connectors)).To(BeZero())
				})
			})
		})
	})
})
