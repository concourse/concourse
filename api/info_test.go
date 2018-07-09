package api_test

import (
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/creds/credhub"
	"github.com/concourse/atc/creds/vault"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {
	Describe("GET /api/v1/info", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/info")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Content-Type 'application/json'", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("contains the version", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`{
				"version": "1.2.3",
				"worker_version": "4.5.6"
			}`))
		})
	})

	Describe("GET /api/v1/info/creds", func() {
		var (
			response   *http.Response
			fakeaccess *accessorfakes.FakeAccess
		)

		BeforeEach(func() {
			fakeaccess = new(accessorfakes.FakeAccess)
		})

		JustBeforeEach(func() {
			fakeAccessor.CreateReturns(fakeaccess)
		})

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("GET", server.URL+"/api/v1/info/creds", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("vault", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAdminReturns(true)

				authConfig := vault.AuthConfig{
					Backend:       "backend-server",
					BackendMaxTTL: 20,
					RetryMax:      5,
					RetryInitial:  2,
				}

				tls := vault.TLS{
					CACert:     "ca-cert",
					ServerName: "server-name",
				}

				vaultManager := &vault.VaultManager{
					URL:        "http://1.2.3.4:8080",
					PathPrefix: "testpath",
					Cache:      false,
					MaxLease:   60,
					TLS:        tls,
					Auth:       authConfig,
				}

				credsManagers["vault"] = vaultManager
			})

			It("returns Content-Type 'application/json'", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns configured creds manager", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
          "vault": {
            "url": "http://1.2.3.4:8080",
            "path_prefix": "testpath",
						"cache": false,
						"max_lease": 60,
            "ca_cert": "ca-cert",
            "server_name": "server-name",
						"auth_backend": "backend-server",
						"auth_max_ttl": 20,
						"auth_retry_max": 5,
						"auth_retry_initial": 2
          }
        }`))
			})
		})

		Context("credhub", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAdminReturns(true)

				tls := credhub.TLS{
					CACerts: []string{"cert1"},
				}
				uaa := credhub.UAA{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
				}
				credhubManager := &credhub.CredHubManager{
					URL:        "http://1.2.3.4:8080",
					PathPrefix: "some-prefix",
					TLS:        tls,
					UAA:        uaa,
				}

				credsManagers["credhub"] = credhubManager
			})

			It("returns Content-Type 'application/json'", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
				Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
			})

			It("returns configured creds manager", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`{
          "credhub": {
            "url": "http://1.2.3.4:8080",
            "path_prefix": "some-prefix",
            "ca_certs": ["cert1"],
						"uaa_client_id": "client-id"
          }
        }`))
			})
		})

	})
})
