package api_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/aws/aws-sdk-go/aws/awserr"
	awssecretsmanager "github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	awsssm "github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/creds/credhub"
	"github.com/concourse/atc/creds/secretsmanager"
	"github.com/concourse/atc/creds/ssm"
	"github.com/concourse/atc/creds/vault"
	vaultapi "github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type MockSsmService struct {
	ssmiface.SSMAPI

	stubGetParameter func(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error)
}

func (m *MockSsmService) GetParameter(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error) {
	return m.stubGetParameter(input)
}

type MockSecretsManagerService struct {
	secretsmanageriface.SecretsManagerAPI

	stubGetSecretValue func(input *awssecretsmanager.GetSecretValueInput) (*awssecretsmanager.GetSecretValueOutput, error)
}

func (m *MockSecretsManagerService) GetSecretValue(input *awssecretsmanager.GetSecretValueInput) (*awssecretsmanager.GetSecretValueOutput, error) {
	return m.stubGetSecretValue(input)
}

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
			credServer *ghttp.Server
			body       []byte
		)

		BeforeEach(func() {
			fakeaccess = new(accessorfakes.FakeAccess)
		})

		JustBeforeEach(func() {
			fakeAccessor.CreateReturns(fakeaccess)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/info/creds", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())

			Expect(response.StatusCode).To(Equal(http.StatusOK))
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))

			body, err = ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("SSM", func() {
			var mockService MockSsmService

			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAdminReturns(true)

				ssmAccess := ssm.NewSsm(lager.NewLogger("ssm_test"), &mockService, "alpha", "bogus", nil)
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
						mockService.stubGetParameter = func(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error) {
							return nil, errors.New("some error occured")
						}
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
						"team_secret_template": "team-secret-template"
          }
        }`))
					})
				})

				Context("get ssm manager info", func() {

					BeforeEach(func() {
						mockService.stubGetParameter = func(input *awsssm.GetParameterInput) (*awsssm.GetParameterOutput, error) {
							return nil, awserr.New(awsssm.ErrCodeParameterNotFound, "dontcare", nil)
						}
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
						"team_secret_template": "team-secret-template"
          }
        }`))
					})
				})
			})
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
					CACert:     "",
					ServerName: "server-name",
				}

				credServer = ghttp.NewServer()
				vaultManager := &vault.VaultManager{
					URL:        credServer.URL(),
					PathPrefix: "testpath",
					Cache:      false,
					MaxLease:   60,
					TLS:        tls,
					Auth:       authConfig,
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
						"some error occured",
					))
				})

				It("returns configured creds manager with error", func() {
					Expect(body).To(MatchJSON(`{
					"vault": {
						"url": "` + credServer.URL() + `",
						"path_prefix": "testpath",
						"cache": false,
						"max_lease": 60,
						"ca_cert": "",
						"server_name": "server-name",
						"auth_backend": "backend-server",
						"auth_max_ttl": 20,
						"auth_retry_max": 5,
						"auth_retry_initial": 2,
						"health": {
							"error": "Error making API request.\n\nURL: GET ` + credServer.URL() + `/v1/sys/health?drsecondarycode=299\u0026sealedcode=299\u0026standbycode=299\u0026uninitcode=299\nCode: 500. Raw Message:\n\n\"some error occured\"",
							"method": "/v1/sys/health"
						}
					}
				}`))
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
						"cache": false,
						"max_lease": 60,
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

		Context("credhub", func() {
			var (
				tls credhub.TLS
				uaa credhub.UAA
			)

			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAdminReturns(true)

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
						PathPrefix  string `json:"path_prefix"`
						UAAClientId string `json:"uaa_client_id"`
					} `json:"credhub"`
				}

				BeforeEach(func() {
					credhubManager := &credhub.CredHubManager{
						URL:        "http://wrong.inexistent.tld",
						PathPrefix: "some-prefix",
						TLS:        tls,
						UAA:        uaa,
						Client:     &credhub.LazyCredhub{},
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
			var mockService MockSecretsManagerService

			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAdminReturns(true)

				secretsManagerAccess := secretsmanager.NewSecretsManager(lager.NewLogger("ssm_test"), &mockService, "alpha", "bogus", nil)

				secretsManager := &secretsmanager.Manager{
					AwsAccessKeyID:         "",
					AwsSecretAccessKey:     "",
					AwsSessionToken:        "",
					AwsRegion:              "blah",
					PipelineSecretTemplate: "pipeline-secret-template",
					TeamSecretTemplate:     "team-secret-template",
					SecretManager:          secretsManagerAccess,
				}

				credsManagers["secretsmanager"] = secretsManager
			})

			Context("returns configured secretsmanager manager", func() {
				Context("get secretsmanager info returns error", func() {

					BeforeEach(func() {
						mockService.stubGetSecretValue = func(input *awssecretsmanager.GetSecretValueInput) (*awssecretsmanager.GetSecretValueOutput, error) {
							return nil, errors.New("some error occurred")
						}
					})

					It("includes the error in json response", func() {
						Expect(body).To(MatchJSON(`{
					"secretsmanager": {
						"aws_region": "blah",
						"pipeline_secret_template": "pipeline-secret-template",
						"team_secret_template": "team-secret-template",
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
						mockService.stubGetSecretValue = func(input *awssecretsmanager.GetSecretValueInput) (*awssecretsmanager.GetSecretValueOutput, error) {

							return nil, awserr.New(awssecretsmanager.ErrCodeResourceNotFoundException, "dontcare", nil)
						}
					})

					It("include sthe secretsmanager info in json response", func() {
						Expect(body).To(MatchJSON(`{
					"secretsmanager": {
						"aws_region": "blah",
						"pipeline_secret_template": "pipeline-secret-template",
						"team_secret_template": "team-secret-template",
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
