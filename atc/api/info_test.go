package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"code.cloudfoundry.org/lager/v3"
	secretsmanagertypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/concourse/concourse/atc/creds/credhub"
	"github.com/concourse/concourse/atc/creds/secretsmanager"
	"github.com/concourse/concourse/atc/creds/secretsmanager/secretsmanagerfakes"
	"github.com/concourse/concourse/atc/creds/ssm"
	"github.com/concourse/concourse/atc/creds/ssm/ssmfakes"
	"github.com/concourse/concourse/atc/creds/vault"
	vaultapi "github.com/hashicorp/vault/api"

	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
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
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
		})

		It("contains the version", func() {
			body, err := io.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(fmt.Sprintf(`{
				"version": "1.2.3",
				"worker_version": "4.5.6",
				"feature_flags": %v,
				"external_url": "https://example.com",
				"cluster_name": "Test Cluster"
			}`, featureFlagsJson)))
		})
	})

	Describe("GET /api/v1/info/creds", func() {
		var (
			response   *http.Response
			credServer *ghttp.Server
			body       []byte
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/info/creds", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))

			body, err = io.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("SSM", func() {
			var mockService *ssmfakes.FakeSsmAPI

			BeforeEach(func() {
				mockService = &ssmfakes.FakeSsmAPI{}

				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)

				ssmAccess := ssm.NewSsm(lager.NewLogger("ssm_test"), mockService, nil, "")
				ssmManager := &ssm.SsmManager{
					AwsAccessKeyID:         "",
					AwsSecretAccessKey:     "",
					AwsSessionToken:        "",
					AwsRegion:              "blah",
					PipelineSecretTemplate: "pipeline-secret-template",
					TeamSecretTemplate:     "team-secret-template",
					Ssm:                    ssmAccess,
				}

				credsManagers["ssm"] = ssmManager
			})

			Context("returns configured ssm manager", func() {
				Context("get ssm manager info returns error", func() {
					BeforeEach(func() {
						mockService.GetParameterReturns(nil, errors.New("some error occured"))
					})

					It("includes the error in json response", func() {
						Expect(body).To(MatchJSON(`{
          "ssm": {
						"aws_region": "blah",
						"health": {
							"error": "some error occured",
							"method": "GetParameter"
						},
						"pipeline_secret_template": "pipeline-secret-template",
            "shared_path": "",
						"team_secret_template": "team-secret-template"
          }
        }`))
					})
				})

				Context("get ssm manager info", func() {
					BeforeEach(func() {
						mockService.GetParameterReturns(nil, &ssmtypes.ParameterNotFound{Message: ptr("dontcare")})
					})

					It("includes the ssm health info in json response", func() {
						Expect(body).To(MatchJSON(`{
          "ssm": {
						"aws_region": "blah",
						"health": {
							"response": {
								"status": "UP"
							},
							"method": "GetParameter"
						},
						"pipeline_secret_template": "pipeline-secret-template",
            "shared_path": "",
						"team_secret_template": "team-secret-template"
          }
        }`))
					})
				})
			})
		})

		Context("vault", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)

				authConfig := vault.AuthConfig{
					Backend:       "backend-server",
					BackendMaxTTL: 20,
					RetryMax:      5,
					RetryInitial:  2,
				}

				tls := vault.TLSConfig{
					CACert:     "",
					ServerName: "server-name",
				}

				credServer = ghttp.NewServer()
				vaultManager := &vault.VaultManager{
					URL:             credServer.URL(),
					Namespace:       "testnamespace",
					PathPrefix:      "testpath",
					LookupTemplates: []string{"/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"},
					TLS:             tls,
					Auth:            authConfig,
				}

				err := vaultManager.Init(lager.NewLogger("test"))
				Expect(err).ToNot(HaveOccurred())

				credsManagers["vault"] = vaultManager

				credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
					http.StatusOK,
					&vaultapi.HealthResponse{
						Initialized:                true,
						Sealed:                     false,
						Standby:                    false,
						ReplicationPerformanceMode: "foo",
						ReplicationDRMode:          "blah",
						ServerTimeUTC:              0,
						Version:                    "1.0.0",
					},
				))
			})

			Context("get vault health info returns error", func() {
				BeforeEach(func() {
					credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
						http.StatusInternalServerError,
						"some error occurred",
					))
				})

				It("returns configured creds manager with error", func() {
					var errorBody struct {
						Vault struct {
							Health struct {
								Error  string `json:"error"`
								Method string `json:"method"`
							} `json:"health"`
						} `json:"vault"`
					}

					err := json.Unmarshal(body, &errorBody)
					Expect(err).ToNot(HaveOccurred())

					Expect(errorBody.Vault.Health.Error).To(ContainSubstring("some error occurred"))
				})
			})

			Context("get vault health info", func() {
				BeforeEach(func() {
					credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
						http.StatusOK,
						&vaultapi.HealthResponse{
							Initialized:                true,
							Sealed:                     false,
							Standby:                    false,
							ReplicationPerformanceMode: "foo",
							ReplicationDRMode:          "blah",
							ServerTimeUTC:              0,
							Version:                    "1.0.0",
						},
					))
				})

				It("returns configured creds manager", func() {
					Expect(body).To(MatchJSON(`{
          "vault": {
            "url": "` + credServer.URL() + `",
            "path_prefix": "testpath",
			"path_prefixes": null,
            "lookup_templates": ["/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"],
			"shared_path": "",
			"namespace": "testnamespace",
            "ca_cert": "",
            "server_name": "server-name",
						"auth_backend": "backend-server",
						"auth_max_ttl": 20,
						"auth_retry_max": 5,
						"auth_retry_initial": 2,
						"health": {
							"response": {
                  "initialized": true,
                  "sealed": false,
                  "standby": false,
				  "performance_standby": false,
                  "replication_performance_mode": "foo",
                  "replication_dr_mode": "blah",
                  "server_time_utc": 0,
                  "version": "1.0.0"
                },
                "method": "/v1/sys/health"
						}
          }
        }`))
				})
			})
		})

		Context("vault with multiple path prefixes", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)

				authConfig := vault.AuthConfig{
					Backend:       "backend-server",
					BackendMaxTTL: 20,
					RetryMax:      5,
					RetryInitial:  2,
				}

				tls := vault.TLSConfig{
					CACert:     "",
					ServerName: "server-name",
				}

				credServer = ghttp.NewServer()
				vaultManager := &vault.VaultManager{
					URL:             credServer.URL(),
					Namespace:       "testnamespace",
					PathPrefix:      "",
					PathPrefixes:    []string{"/kv1", "/kv2"},
					LookupTemplates: []string{"/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"},
					TLS:             tls,
					Auth:            authConfig,
				}

				err := vaultManager.Init(lager.NewLogger("test"))
				Expect(err).ToNot(HaveOccurred())

				credsManagers["vault"] = vaultManager

				credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
					http.StatusOK,
					&vaultapi.HealthResponse{
						Initialized:                true,
						Sealed:                     false,
						Standby:                    false,
						ReplicationPerformanceMode: "foo",
						ReplicationDRMode:          "blah",
						ServerTimeUTC:              0,
						Version:                    "1.0.0",
					},
				))
			})

			Context("get vault health info returns error", func() {
				BeforeEach(func() {
					credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
						http.StatusInternalServerError,
						"some error occurred",
					))
				})

				It("returns configured creds manager with error", func() {
					var errorBody struct {
						Vault struct {
							Health struct {
								Error  string `json:"error"`
								Method string `json:"method"`
							} `json:"health"`
						} `json:"vault"`
					}

					err := json.Unmarshal(body, &errorBody)
					Expect(err).ToNot(HaveOccurred())

					Expect(errorBody.Vault.Health.Error).To(ContainSubstring("some error occurred"))
				})
			})

			Context("get vault health info", func() {
				BeforeEach(func() {
					credServer.RouteToHandler("GET", "/v1/sys/health", ghttp.RespondWithJSONEncoded(
						http.StatusOK,
						&vaultapi.HealthResponse{
							Initialized:                true,
							Sealed:                     false,
							Standby:                    false,
							ReplicationPerformanceMode: "foo",
							ReplicationDRMode:          "blah",
							ServerTimeUTC:              0,
							Version:                    "1.0.0",
						},
					))
				})

				It("returns configured creds manager", func() {
					Expect(body).To(MatchJSON(`{
          "vault": {
            "url": "` + credServer.URL() + `",
            "path_prefix": "",
			"path_prefixes": ["/kv1","/kv2"],
            "lookup_templates": ["/{{.Team}}/{{.Pipeline}}/{{.Secret}}", "/{{.Team}}/{{.Secret}}"],
			"shared_path": "",
			"namespace": "testnamespace",
            "ca_cert": "",
            "server_name": "server-name",
			"auth_backend": "backend-server",
			"auth_max_ttl": 20,
			"auth_retry_max": 5,
			"auth_retry_initial": 2,
			"health": {
				"response": {
                  "initialized": true,
                  "sealed": false,
                  "standby": false,
				  "performance_standby": false,
                  "replication_performance_mode": "foo",
                  "replication_dr_mode": "blah",
                  "server_time_utc": 0,
                  "version": "1.0.0",
				  "enterprise": false,
				  "echo_duration_ms": 0,
				  "clock_skew_ms": 0,
				  "replication_primary_canary_age_ms": 0
                },
                "method": "/v1/sys/health"
			}
          }
        }`))
				})
			})
		})

		Context("credhub", func() {
			var (
				tls credhub.TLS
				uaa credhub.UAA
			)

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)

				tls = credhub.TLS{
					CACerts: []string{},
				}
				uaa = credhub.UAA{
					ClientId:     "client-id",
					ClientSecret: "client-secret",
				}
			})

			Context("get credhub help info succeeds", func() {
				BeforeEach(func() {
					credServer = ghttp.NewServer()
					credServer.RouteToHandler("GET", "/health", ghttp.RespondWithJSONEncoded(
						http.StatusOK, map[string]string{
							"status": "UP",
						},
					))

					credhubManager := &credhub.CredHubManager{
						URL:        credServer.URL(),
						PathPrefix: "some-prefix",
						TLS:        tls,
						UAA:        uaa,
						Client:     &credhub.LazyCredhub{},
					}

					credsManagers["credhub"] = credhubManager
				})

				It("returns configured creds manager with response", func() {
					Expect(body).To(MatchJSON(`{
					"credhub": {
						"url": "` + credServer.URL() + `",
						"ca_certs": [],
						"health": {
							"response": {
								"status": "UP"
							},
							"method": "/health"
						},
						"path_prefix": "some-prefix",
						"path_prefixes": null,
						"uaa_client_id": "client-id"
						}
					}`))
				})
			})

			Context("get credhub health info returns error", func() {
				type responseSkeleton struct {
					CredHub struct {
						Url     string   `json:"url"`
						CACerts []string `json:"ca_certs"`
						Health  struct {
							Error    string `json:"error"`
							Response struct {
								Status string `json:"status"`
							} `json:"response"`
							Method string `json:"method"`
						} `json:"health"`
						PathPrefix   string   `json:"path_prefix"`
						PathPrefixes []string `json:"path_prefixes"`
						UAAClientId  string   `json:"uaa_client_id"`
					} `json:"credhub"`
				}

				BeforeEach(func() {
					credhubManager := &credhub.CredHubManager{
						URL:          "http://wrong.inexistent.tld",
						PathPrefix:   "some-prefix",
						PathPrefixes: []string{},
						TLS:          tls,
						UAA:          uaa,
						Client:       &credhub.LazyCredhub{},
					}

					credsManagers["credhub"] = credhubManager
				})

				It("returns configured creds manager with error", func() {
					var parsedResponse responseSkeleton

					err := json.Unmarshal(body, &parsedResponse)
					Expect(err).ToNot(HaveOccurred())

					Expect(parsedResponse.CredHub.Url).To(Equal("http://wrong.inexistent.tld"))
					Expect(parsedResponse.CredHub.CACerts).To(BeEmpty())
					Expect(parsedResponse.CredHub.PathPrefix).To(Equal("some-prefix"))
					Expect(parsedResponse.CredHub.UAAClientId).To(Equal("client-id"))
					Expect(parsedResponse.CredHub.Health.Response).ToNot(BeNil())
					Expect(parsedResponse.CredHub.Health.Response.Status).To(BeEmpty())
					Expect(parsedResponse.CredHub.Health.Method).To(Equal("/health"))
					Expect(parsedResponse.CredHub.Health.Error).To(ContainSubstring("no such host"))
				})
			})
		})

		Context("SecretsManager", func() {
			var mockService *secretsmanagerfakes.FakeSecretsManagerAPI

			BeforeEach(func() {
				mockService = &secretsmanagerfakes.FakeSecretsManagerAPI{}

				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAdminReturns(true)

				secretsManagerAccess := secretsmanager.NewSecretsManager(lager.NewLogger("ssm_test"), mockService, nil)

				secretsManager := &secretsmanager.Manager{
					AwsAccessKeyID:         "",
					AwsSecretAccessKey:     "",
					AwsSessionToken:        "",
					AwsRegion:              "blah",
					PipelineSecretTemplate: "pipeline-secret-template",
					TeamSecretTemplate:     "team-secret-template",
					SharedSecretTemplate:   "shared-secret-template",
					SecretManager:          secretsManagerAccess,
				}

				credsManagers["secretsmanager"] = secretsManager
			})

			Context("returns configured secretsmanager manager", func() {
				Context("get secretsmanager info returns error", func() {
					BeforeEach(func() {
						mockService.GetSecretValueReturns(nil, errors.New("some error occurred"))
					})

					It("includes the error in json response", func() {
						Expect(body).To(MatchJSON(`{
					"secretsmanager": {
						"aws_region": "blah",
						"pipeline_secret_template": "pipeline-secret-template",
						"team_secret_template": "team-secret-template",
						"shared_secret_template": "shared-secret-template",
						"health": {
							"error": "some error occurred",
							"method": "GetSecretValue"
						}
					}
				}`))
					})

				})

				Context("get secretsmanager info", func() {
					BeforeEach(func() {
						mockService.GetSecretValueReturns(nil, &secretsmanagertypes.ResourceNotFoundException{Message: ptr("dontcare")})
					})

					It("include sthe secretsmanager info in json response", func() {
						Expect(body).To(MatchJSON(`{
					"secretsmanager": {
						"aws_region": "blah",
						"pipeline_secret_template": "pipeline-secret-template",
						"team_secret_template": "team-secret-template",
						"shared_secret_template": "shared-secret-template",
						"health": {
							"response": {
								"status": "UP"
							},
							"method": "GetSecretValue"
						}
					}
				}`))
					})
				})
			})

		})
	})
})

func ptr[T any](v T) *T {
	return &v
}
