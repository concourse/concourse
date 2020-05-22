package vault_test

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/creds/vault"
	"github.com/hashicorp/vault/api"
	"github.com/jessevdk/go-flags"
	"github.com/square/certstrap/pkix"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("VaultManager", func() {
	var manager vault.VaultManager

	Describe("IsConfigured()", func() {
		JustBeforeEach(func() {
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
		})

		It("fails on empty Manager", func() {
			Expect(manager.IsConfigured()).To(BeFalse())
		})

		It("passes if URL is set", func() {
			manager.URL = "http://vault"
			Expect(manager.IsConfigured()).To(BeTrue())
		})
	})

	Describe("Validate()", func() {
		JustBeforeEach(func() {
			manager = vault.VaultManager{URL: "http://vault", Auth: vault.AuthConfig{ClientToken: "xxx"}}
			_, err := flags.ParseArgs(&manager, []string{})
			Expect(err).To(BeNil())
			Expect(manager.SharedPath).To(Equal(""))
			Expect(manager.PathPrefix).To(Equal("/concourse"))
			Expect(manager.LookupTemplates).To(Equal([]string{
				"/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
				"/{{.Team}}/{{.Secret}}",
			}))
			Expect(manager.Namespace).To(Equal(""))
		})

		It("passes on default parameters", func() {
			Expect(manager.Validate()).To(BeNil())
		})

		DescribeTable("passes if all vault credentials are specified",
			func(backend, clientToken string) {
				manager.Auth.Backend = backend
				manager.Auth.ClientToken = clientToken
				Expect(manager.Validate()).To(BeNil())
			},
			Entry("all values", "backend", "clientToken"),
			Entry("only clientToken", "", "clientToken"),
		)

		It("fails on missing vault auth credentials", func() {
			manager.Auth = vault.AuthConfig{}
			Expect(manager.Validate()).ToNot(BeNil())
		})
	})

	Describe("Config", func() {
		var config map[string]interface{}
		var fakeVault *httptest.Server

		var configErr error

		BeforeEach(func() {
			key, err := pkix.CreateRSAKey(1024)
			Expect(err).ToNot(HaveOccurred())

			ca, err := pkix.CreateCertificateAuthority(key, "", time.Now().Add(time.Hour), "", "", "", "", "vault-ca")
			Expect(err).ToNot(HaveOccurred())

			serverKey, err := pkix.CreateRSAKey(1024)
			Expect(err).ToNot(HaveOccurred())

			serverKeyBytes, err := serverKey.ExportPrivate()
			Expect(err).ToNot(HaveOccurred())

			serverName := "vault"

			serverCSR, err := pkix.CreateCertificateSigningRequest(serverKey, "", []net.IP{net.ParseIP("127.0.0.1")}, []string{serverName}, "", "", "", "", "")
			Expect(err).ToNot(HaveOccurred())

			serverCert, err := pkix.CreateCertificateHost(ca, key, serverCSR, time.Now().Add(time.Hour))
			Expect(err).ToNot(HaveOccurred())

			clientKey, err := pkix.CreateRSAKey(1024)
			Expect(err).ToNot(HaveOccurred())

			clientKeyBytes, err := clientKey.ExportPrivate()
			Expect(err).ToNot(HaveOccurred())

			clientCSR, err := pkix.CreateCertificateSigningRequest(clientKey, "", nil, nil, "", "", "", "", "concourse")
			Expect(err).ToNot(HaveOccurred())

			clientCert, err := pkix.CreateCertificateHost(ca, key, clientCSR, time.Now().Add(time.Hour))
			Expect(err).ToNot(HaveOccurred())

			serverCertBytes, err := serverCert.Export()
			Expect(err).ToNot(HaveOccurred())

			clientCertBytes, err := clientCert.Export()
			Expect(err).ToNot(HaveOccurred())

			caBytes, err := ca.Export()
			Expect(err).ToNot(HaveOccurred())

			fakeVault = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err := json.NewEncoder(w).Encode(api.Secret{
					Data: map[string]interface{}{"value": "foo"},
				})
				Expect(err).ToNot(HaveOccurred())
			}))

			tlsCert, err := tls.X509KeyPair(serverCertBytes, serverKeyBytes)
			Expect(err).ToNot(HaveOccurred())

			fakeVault.TLS = &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
			}

			fakeVault.StartTLS()

			config = map[string]interface{}{
				"url":                  fakeVault.URL,
				"path_prefix":          "/path-prefix",
				"lookup_templates": []string{
					"/what/{{.Team}}/blah/{{.Pipeline}}/{{.Secret}}",
					"/thing/{{.Team}}/{{.Secret}}",
				},
				"shared_path":          "/shared-path",
				"namespace":            "some-namespace",
				"ca_cert":              string(caBytes),
				"client_cert":          string(clientCertBytes),
				"client_key":           string(clientKeyBytes),
				"server_name":          serverName,
				"insecure_skip_verify": true,
				"client_token":         "some-client-token",
				"auth_backend_max_ttl": "5m",
				"auth_retry_max":       "15m",
				"auth_retry_initial":   "10s",
				"auth_backend":         "approle",
				"auth_params": map[string]string{
					"role_id":   "some-role-id",
					"secret_id": "some-secret-id",
				},
			}

			manager = vault.VaultManager{}
		})

		JustBeforeEach(func() {
			configErr = manager.Config(config)
		})

		It("configures TLS appropriately", func() {
			Expect(configErr).ToNot(HaveOccurred())

			err := manager.Init(lagertest.NewTestLogger("test"))
			Expect(err).ToNot(HaveOccurred())

			secret, err := manager.Client.Read("some/path")
			Expect(err).ToNot(HaveOccurred())
			Expect(secret).ToNot(BeNil())
			Expect(secret.Data).To(Equal(map[string]interface{}{"value": "foo"}))
		})

		It("configures all attributes appropriately", func() {
			Expect(configErr).ToNot(HaveOccurred())

			Expect(manager.URL).To(Equal(fakeVault.URL))
			Expect(manager.PathPrefix).To(Equal("/path-prefix"))
			Expect(manager.LookupTemplates).To(Equal([]string{
				"/what/{{.Team}}/blah/{{.Pipeline}}/{{.Secret}}",
				"/thing/{{.Team}}/{{.Secret}}",
			}))
			Expect(manager.SharedPath).To(Equal("/shared-path"))
			Expect(manager.Namespace).To(Equal("some-namespace"))

			Expect(manager.TLS.Insecure).To(BeTrue())

			Expect(manager.Auth.ClientToken).To(Equal("some-client-token"))
			Expect(manager.Auth.BackendMaxTTL).To(Equal(5 * time.Minute))
			Expect(manager.Auth.RetryMax).To(Equal(15 * time.Minute))
			Expect(manager.Auth.RetryInitial).To(Equal(10 * time.Second))
			Expect(manager.Auth.Backend).To(Equal("approle"))
			Expect(manager.Auth.Params).To(Equal(map[string]string{
				"role_id":   "some-role-id",
				"secret_id": "some-secret-id",
			}))
		})

		Context("with optional configs omitted", func() {
			BeforeEach(func() {
				delete(config, "path_prefix")
				delete(config, "auth_retry_max")
				delete(config, "auth_retry_initial")
				delete(config, "lookup_templates")
			})

			It("has sane defaults", func() {
				Expect(configErr).ToNot(HaveOccurred())

				Expect(manager.PathPrefix).To(Equal("/concourse"))
				Expect(manager.Auth.RetryMax).To(Equal(5 * time.Minute))
				Expect(manager.Auth.RetryInitial).To(Equal(time.Second))
				Expect(manager.LookupTemplates).To(Equal([]string{
					"/{{.Team}}/{{.Pipeline}}/{{.Secret}}",
					"/{{.Team}}/{{.Secret}}",
				}))
			})
		})

		Context("with extra keys in the config", func() {
			BeforeEach(func() {
				config["unknown_key"] = "whambam"
			})

			It("returns an error", func() {
				Expect(configErr).To(HaveOccurred())
				Expect(configErr.Error()).To(ContainSubstring("unknown_key"))
			})
		})
	})
})
