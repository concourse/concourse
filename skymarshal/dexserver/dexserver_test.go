package dexserver_test

import (
	"errors"
	"sort"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/skymarshal/dexserver"
	"github.com/concourse/concourse/skymarshal/skycmd"
	store "github.com/concourse/concourse/skymarshal/storage"
	"github.com/concourse/dex/server"
	"github.com/concourse/dex/storage"
	"github.com/concourse/flag"
	"golang.org/x/crypto/bcrypt"
)

var _ = Describe("Dex Server", func() {
	var config *dexserver.DexConfig
	var serverConfig server.Config
	var storage storage.Storage
	var logger lager.Logger
	var err error

	successful := true

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
		if successful {
			Expect(err).ToNot(HaveOccurred())
		}
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
					config.Users = map[string]string{
						"some-user-0": "$2a$10$3veRX245rLrpOKrgu7jIyOEKF5Km5tY86bZql6/oTMssgPO/6XJju",
						"some-user-1": "$2a$10$31qaZYMqx7mplkLoMrpPHeF3xf5eN37Zyv3e/QdPUs6S6IqrDA9Du",
					}
				})

				ConfiguresUsersCorrectly()
			})

			Context("when the user's password is provided in plaintext", func() {
				BeforeEach(func() {
					config.Users = map[string]string{
						"some-user-0": "some-password-0",
						"some-user-1": "some-password-1",
					}
				})

				ConfiguresUsersCorrectly()

				Context("when a user's password is changed", func() {
					BeforeEach(func() {
						// First create the first config based on the parent Context
						serverConfig, err = dexserver.NewDexServerConfig(config)
						Expect(err).ToNot(HaveOccurred())

						// The final config will be created in the JustBeforeEach block
						config.Users = map[string]string{
							"some-user-0": "some-password-0",
							"some-user-1": "some-password-1-changed",
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
						config.Users = map[string]string{
							"some-user-0": "some-password-0",
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

		Context("when auth provider is configured", func() {
			BeforeEach(func() {
				config.Connectors = skycmd.ConnectorsConfig{
					BitbucketCloud: skycmd.BitbucketCloudFlags{
						Enabled: true,
					},
				}
			})

			Context("with invalid configuration", func() {
				BeforeEach(func() {
					config.Connectors.BitbucketCloud.ClientID = "client-id"
					successful = false
				})

				It("fails to create dexserver because of failed validation", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(&multierror.Error{Errors: []error{errors.New("Missing client-secret")}}))
				})
			})

			Context("with valid configuration", func() {
				BeforeEach(func() {
					config.Connectors.BitbucketCloud.ClientID = "client-id"
					config.Connectors.BitbucketCloud.ClientSecret = "client-secret"
				})

				It("successfully adds the auth provider", func() {
					connectors, err := storage.ListConnectors()
					Expect(err).NotTo(HaveOccurred())

					bitbucket := skycmd.BitbucketCloudFlags{}
					Expect(connectors).To(HaveLen(1))
					Expect(connectors[0].ID).To(Equal(bitbucket.ID()))
					Expect(connectors[0].Type).To(Equal(bitbucket.ID()))
					Expect(connectors[0].Name).To(Equal(bitbucket.Name()))
				})
			})
		})

		Context("when clients are configured in plain text", func() {
			BeforeEach(func() {
				config.Clients = map[string]string{
					"some-client-id": "some-client-secret",
				}
				config.RedirectURL = "http://example.com"
			})

			It("should contain the configured clients with a bcrypted secret", func() {
				clients, err := storage.ListClients()
				Expect(err).NotTo(HaveOccurred())
				Expect(clients).To(HaveLen(1))
				Expect(clients[0].ID).To(Equal("some-client-id"))
				Expect(bcrypt.CompareHashAndPassword([]byte(clients[0].Secret), []byte("some-client-secret"))).NotTo(HaveOccurred())
				Expect(clients[0].RedirectURIs).To(ContainElement("http://example.com"))
			})
		})

		Context("when clients are configured in bcrypt format", func() {
			BeforeEach(func() {
				config.Clients = map[string]string{
					"some-client-id": "$2a$10$3veRX245rLrpOKrgu7jIyOEKF5Km5tY86bZql6/oTMssgPO/6XJju",
				}
				config.RedirectURL = "http://example.com"
			})

			It("should contain the configured clients with the given secret", func() {
				clients, err := storage.ListClients()
				Expect(err).NotTo(HaveOccurred())
				Expect(clients).To(HaveLen(1))
				Expect(clients[0].ID).To(Equal("some-client-id"))
				Expect(clients[0].Secret).To(Equal("$2a$10$3veRX245rLrpOKrgu7jIyOEKF5Km5tY86bZql6/oTMssgPO/6XJju"))
				Expect(clients[0].RedirectURIs).To(ContainElement("http://example.com"))
			})
		})
	})
})
