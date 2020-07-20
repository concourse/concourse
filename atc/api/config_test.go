package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds/noop"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/rata"
	"sigs.k8s.io/yaml"

	// load dummy credential manager
	_ "github.com/concourse/concourse/atc/creds/dummy"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config API", func() {
	var (
		pipelineConfig   atc.Config
		requestGenerator *rata.RequestGenerator
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)

		pipelineConfig = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"some-job"},
					Resources: []string{"some-resource"},
				},
			},

			VarSources: atc.VarSourceConfigs{
				{
					Name: "some",
					Type: "dummy",
					Config: map[string]interface{}{
						"vars": map[string]interface{}{},
					},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
					},
				},
			},

			ResourceTypes: atc.ResourceTypes{
				{
					Name:   "custom-resource",
					Type:   "custom-type",
					Source: atc.Source{"custom": "source"},
					Tags:   atc.Tags{"some-tag"},
				},
			},

			Jobs: atc.JobConfigs{
				{
					Name:   "some-job",
					Public: true,
					Serial: true,
					PlanSequence: []atc.Step{
						{
							Config: &atc.GetStep{
								Name:     "some-input",
								Resource: "some-resource",
								Params:   atc.Params{"some-param": "some-value"},
							},
						},
						{
							Config: &atc.TaskStep{
								Name:       "some-task",
								Privileged: true,
								Config: &atc.TaskConfig{
									Platform:  "linux",
									RootfsURI: "some-image",
									Run: atc.TaskRunConfig{
										Path: "/path/to/run",
									},
								},
							},
						},
						{
							Config: &atc.PutStep{
								Name:     "some-output",
								Resource: "some-resource",
								Params:   atc.Params{"some-param": "some-value"},
							},
						},
					},
				},
			},
		}
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:name/config", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			req, err := requestGenerator.CreateRequest(atc.GetConfig, rata.Params{
				"team_name":     "a-team",
				"pipeline_name": "something-else",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when the team is found", func() {
				var fakeTeam *dbfakes.FakeTeam
				BeforeEach(func() {
					fakeTeam = new(dbfakes.FakeTeam)
					fakeTeam.NameReturns("a-team")
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				Context("when the pipeline is found", func() {
					var fakePipeline *dbfakes.FakePipeline
					BeforeEach(func() {
						fakePipeline = new(dbfakes.FakePipeline)
						fakePipeline.NameReturns("something-else")
						fakePipeline.ConfigVersionReturns(1)
						fakePipeline.GroupsReturns(atc.GroupConfigs{
							{
								Name:      "some-group",
								Jobs:      []string{"some-job"},
								Resources: []string{"some-resource"},
							},
						})
						fakePipeline.VarSourcesReturns(atc.VarSourceConfigs{
							{
								Name: "some",
								Type: "dummy",
								Config: map[string]interface{}{
									"vars": map[string]interface{}{},
								},
							},
						})
						fakeTeam.PipelineReturns(fakePipeline, true, nil)
					})

					Context("when the pipeline config is found", func() {
						BeforeEach(func() {
							fakePipeline.ConfigReturns(pipelineConfig, nil)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns Content-Type 'application/json' and config version as X-Concourse-Config-Version", func() {
							expectedHeaderEntries := map[string]string{
								"Content-Type":          "application/json",
								atc.ConfigVersionHeader: "1",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
						})

						It("returns the config", func() {
							var actualConfigResponse atc.ConfigResponse
							err := json.NewDecoder(response.Body).Decode(&actualConfigResponse)
							Expect(err).NotTo(HaveOccurred())

							Expect(actualConfigResponse).To(Equal(atc.ConfigResponse{
								Config: pipelineConfig,
							}))
						})

						Context("when finding the config fails", func() {
							BeforeEach(func() {
								fakePipeline.ConfigReturns(atc.Config{}, errors.New("fail"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the pipeline is archived", func() {
						BeforeEach(func() {
							fakePipeline.ArchivedReturns(true)
						})
						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})
				})

				Context("when the pipeline is not found", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the pipeline fails", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(nil, false, errors.New("failed"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when the team is not found", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when finding the team fails", func() {
				BeforeEach(func() {
					dbTeamFactory.FindTeamReturns(nil, false, errors.New("failed"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:name/config", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error
			request, err = requestGenerator.CreateRequest(atc.SaveConfig, rata.Params{
				"team_name":     "a-team",
				"pipeline_name": "a-pipeline",
			}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when an identifier is invalid", func() {

				BeforeEach(func() {
					var err error
					request, err = requestGenerator.CreateRequest(atc.SaveConfig, rata.Params{
						"team_name":     "_team",
						"pipeline_name": "_pipeline",
					}, nil)
					Expect(err).NotTo(HaveOccurred())

					request.Header.Set("Content-Type", "application/json")

					payload, err := json.Marshal(pipelineConfig)
					Expect(err).NotTo(HaveOccurred())

					request.Body = gbytes.BufferWithBytes(payload)
				})

				It("returns warnings in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"warnings": [
									{
										"type": "invalid_identifier",
										"message": "pipeline: '_pipeline' is not a valid identifier: must start with a lowercase letter"
									},
									{
										"type": "invalid_identifier",
										"message": "team: '_team' is not a valid identifier: must start with a lowercase letter"
									}
								]
							}`))
				})
			})

			Context("when a config version is specified", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigVersionHeader, "42")
				})

				Context("when the config is malformed", func() {
					Context("JSON", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/json")
							request.Body = gbytes.BufferWithBytes([]byte(`{`))
						})

						It("returns 400", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("returns Content-Type 'application/json'", func() {
							expectedHeaderEntries := map[string]string{
								"Content-Type": "application/json",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
						})

						It("returns error JSON", func() {
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"malformed config: error converting YAML to JSON: yaml: line 1: did not find expected node content"
									]
								}`))
						})

						It("does not save anything", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
						})
					})

					Context("YAML", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/x-yaml")
							request.Body = gbytes.BufferWithBytes([]byte(`{`))
						})

						It("returns 400", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						})

						It("returns Content-Type 'application/json'", func() {
							expectedHeaderEntries := map[string]string{
								"Content-Type": "application/json",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
						})

						It("returns error JSON", func() {
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"malformed config: error converting YAML to JSON: yaml: line 1: did not find expected node content"
									]
								}`))
						})

						It("does not save anything", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
						})
					})
				})

				Context("when the config is valid", func() {
					Context("JSON", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/json")

							payload, err := json.Marshal(pipelineConfig)
							Expect(err).NotTo(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("notifies the scanner to run", func() {
							Expect(dbTeamFactory.NotifyResourceScannerCallCount()).To(Equal(1))
						})

						It("returns Content-Type 'application/json'", func() {
							expectedHeaderEntries := map[string]string{
								"Content-Type": "application/json",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
						})

						It("saves it initially paused", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, initiallyPaused := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(initiallyPaused).To(BeTrue())
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								dbTeam.SavePipelineReturns(nil, false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})

							It("returns the error in the response body", func() {
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("failed to save config: oh no!")))
							})
						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								returnedPipeline := new(dbfakes.FakePipeline)
								dbTeam.SavePipelineReturns(returnedPipeline, true, nil)
							})

							It("returns 201", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})

							It("does not notify the scanner to run", func() {
								Expect(dbTeamFactory.NotifyResourceScannerCallCount()).To(Equal(0))
							})
						})

						Context("when the config is invalid", func() {
							BeforeEach(func() {
								pipelineConfig.Groups[0].Resources = []string{"missing-resource"}
								payload, err := json.Marshal(pipelineConfig)
								Expect(err).NotTo(HaveOccurred())
								request.Body = gbytes.BufferWithBytes(payload)
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})

							It("returns Content-Type 'application/json'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns error JSON", func() {
								Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"invalid groups:\n\tgroup 'some-group' has unknown resource 'missing-resource'\n"
									]
								}`))
							})

							It("does not save it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
							})
						})
					})

					Context("YAML", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/x-yaml")

							payload, err := yaml.Marshal(pipelineConfig)
							Expect(err).NotTo(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 200", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("notifies the scanner to run", func() {
							Expect(dbTeamFactory.NotifyResourceScannerCallCount()).To(Equal(1))
						})

						It("returns Content-Type 'application/json'", func() {
							expectedHeaderEntries := map[string]string{
								"Content-Type": "application/json",
							}
							Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
						})

						It("saves it initially paused", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, initiallyPaused := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(initiallyPaused).To(BeTrue())
						})

						Context("when the payload contains suspicious types", func() {
							BeforeEach(func() {
								payload := `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
  check_timeout: 1m
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    config:
      platform: linux
      run:
        path: ls
      params:
        FOO: true
        BAR: 1
        BAZ: 1.9`

								request.Header.Set("Content-Type", "application/x-yaml")
								request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
							})

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns Content-Type 'application/json'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("saves it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

								name, savedConfig, id, initiallyPaused := dbTeam.SavePipelineArgsForCall(0)
								Expect(name).To(Equal("a-pipeline"))
								Expect(savedConfig).To(Equal(atc.Config{
									Resources: []atc.ResourceConfig{
										{
											Name:         "some-resource",
											Type:         "some-type",
											Source:       nil,
											CheckEvery:   "10s",
											CheckTimeout: "1m",
										},
									},
									Jobs: atc.JobConfigs{
										{
											Name: "some-job",
											PlanSequence: []atc.Step{
												{
													Config: &atc.GetStep{
														Name: "some-resource",
													},
												},
												{
													Config: &atc.TaskStep{
														Name: "some-task",
														Config: &atc.TaskConfig{
															Platform: "linux",

															Run: atc.TaskRunConfig{
																Path: "ls",
															},

															Params: atc.TaskEnv{
																"FOO": "true",
																"BAR": "1",
																"BAZ": "1.9",
															},
														},
													},
												},
											},
										},
									},
								}))
								Expect(id).To(Equal(db.ConfigVersion(42)))
								Expect(initiallyPaused).To(BeTrue())
							})
						})

						Describe("test validate cred params when the check_creds param is set in request", func() {
							var (
								payload string
							)

							BeforeEach(func() {
								query := request.URL.Query()
								query.Add(atc.SaveConfigCheckCreds, "")
								request.URL.RawQuery = query.Encode()
							})

							ExpectCredsValidationPass := func() {
								Context("when the param exists in creds manager", func() {
									BeforeEach(func() {
										fakeSecretManager.GetReturns("this-string-value-doesn't-matter", nil, true, nil)
									})

									It("passes validation", func() {
										Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))
									})

									It("returns 200 ok", func() {
										Expect(response.StatusCode).To(Equal(http.StatusOK))
									})
								})
							}

							ExpectCredsValidationFail := func() {
								Context("when the param does not exist in creds manager", func() {
									BeforeEach(func() {
										fakeSecretManager.GetReturns(nil, nil, false, nil)
									})

									It("fail validation", func() {
										Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
									})

									It("returns 400", func() {
										Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
									})

								})
							}
							Context("when there is param in resource type config", func() {
								BeforeEach(func() {
									payload = `---
resource_types:
- name: some-type
  type: some-base-resource-type
  source:
    FOO: ((BAR))`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})

							Context("when there is param in resource source config", func() {
								BeforeEach(func() {
									payload = `---
resources:
- name: some-resource
  type: some-type
  source:
    FOO: ((BAR))
jobs:
- name: some-job
  plan:
  - get: some-resource`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})

							Context("when there is param in resource webhook token", func() {
								BeforeEach(func() {
									payload = `---
resources:
- name: some-resource
  type: some-type
  webhook_token: ((BAR))
jobs:
- name: some-job
  plan:
  - get: some-resource`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})

							Context("when it contains task that uses external config file and params in task params", func() {
								BeforeEach(func() {
									payload = `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    file: some-resource/config.yml
    params:
      FOO: ((BAR))`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})

							Context("when it contains task that uses external config file and params in task vars", func() {
								BeforeEach(func() {
									payload = `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    file: some-resource/config.yml
    vars:
      FOO: ((BAR))`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})

							Context("when it contains nested task that uses external config file and params in task vars", func() {
								BeforeEach(func() {
									payload = `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
jobs:
- name: some-job
  plan:
  - get: some-resource
  - do:
    - task: some-task
      file: some-resource/config.yml
      vars:
        FOO: ((BAR))`

									request.Header.Set("Content-Type", "application/x-yaml")
									request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
								})

								ExpectCredsValidationPass()
								ExpectCredsValidationFail()
							})
						})

						Context("when it contains credentials to be interpolated", func() {
							var (
								payloadAsConfig atc.Config
								payload         string
							)

							BeforeEach(func() {
								payload = `---
resources:
- name: some-resource
  type: some-type
  check_every: 10s
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    config:
      platform: linux
      run:
        path: ls
      params:
        FOO: ((BAR))`
								payloadAsConfig = atc.Config{Resources: []atc.ResourceConfig{
									{
										Name:       "some-resource",
										Type:       "some-type",
										Source:     nil,
										CheckEvery: "10s",
									},
								},
									Jobs: atc.JobConfigs{
										{
											Name: "some-job",
											PlanSequence: []atc.Step{
												{
													Config: &atc.GetStep{
														Name: "some-resource",
													},
												},
												{
													Config: &atc.TaskStep{
														Name: "some-task",
														Config: &atc.TaskConfig{
															Platform: "linux",

															Run: atc.TaskRunConfig{
																Path: "ls",
															},

															Params: atc.TaskEnv{
																"FOO": "((BAR))",
															},
														},
													},
												},
											},
										},
									},
								}

								request.Header.Set("Content-Type", "application/x-yaml")
								request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
							})

							Context("when the check_creds param is set", func() {
								BeforeEach(func() {
									query := request.URL.Query()
									query.Add(atc.SaveConfigCheckCreds, "")
									request.URL.RawQuery = query.Encode()
								})

								Context("when the credential exists in the credential manager", func() {
									BeforeEach(func() {
										fakeSecretManager.GetReturns("this-string-value-doesn't-matter", nil, true, nil)
									})

									It("passes validation and saves it un-interpolated", func() {
										Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

										name, savedConfig, id, initiallyPaused := dbTeam.SavePipelineArgsForCall(0)
										Expect(name).To(Equal("a-pipeline"))
										Expect(savedConfig).To(Equal(payloadAsConfig))
										Expect(id).To(Equal(db.ConfigVersion(42)))
										Expect(initiallyPaused).To(BeTrue())
									})

									It("returns 200", func() {
										Expect(response.StatusCode).To(Equal(http.StatusOK))
									})
								})

								Context("when the credential does not exist in the credential manager", func() {
									BeforeEach(func() {
										fakeSecretManager.GetReturns(nil, nil, false, nil) // nil value, nil expiration, not found, no error
									})

									It("returns 400", func() {
										Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
									})

									It("returns the credential name that was missing", func() {
										Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"errors":["credential validation failed\n\n1 error occurred:\n\t* failed to interpolate task config: undefined vars: BAR\n\n"]}`))
									})
								})

								Context("when a credentials manager is not used", func() {
									BeforeEach(func() {
										fakeSecretManager.GetStub = func(secretPath string) (interface{}, *time.Time, bool, error) {
											return noop.Noop{}.Get(secretPath)
										}
									})

									It("returns 400", func() {
										Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
									})

									It("returns the credential name that was missing", func() {
										Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"errors":["credential validation failed\n\n1 error occurred:\n\t* failed to interpolate task config: undefined vars: BAR\n\n"]}`))
									})
								})
							})

						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								returnedPipeline := new(dbfakes.FakePipeline)
								dbTeam.SavePipelineReturns(returnedPipeline, true, nil)
							})

							It("returns 201", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})

							It("does not notify the scanner to run", func() {
								Expect(dbTeamFactory.NotifyResourceScannerCallCount()).To(Equal(0))
							})
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								dbTeam.SavePipelineReturns(nil, false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})

							It("returns the error in the response body", func() {
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("failed to save config: oh no!")))
							})
						})

						Context("when the config is invalid", func() {
							BeforeEach(func() {
								pipelineConfig.Groups[0].Resources = []string{"missing-resource"}
								payload, err := json.Marshal(pipelineConfig)
								Expect(err).NotTo(HaveOccurred())
								request.Body = gbytes.BufferWithBytes(payload)
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})

							It("returns Content-Type 'application/json'", func() {
								expectedHeaderEntries := map[string]string{
									"Content-Type": "application/json",
								}
								Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
							})

							It("returns error JSON", func() {
								Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"invalid groups:\n\tgroup 'some-group' has unknown resource 'missing-resource'\n"
									]
								}`))
							})

							It("does not save it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(BeZero())
							})
						})
					})
				})

				Context("when the Content-Type is unsupported", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/x-toml")

						payload, err := yaml.Marshal(pipelineConfig)
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(payload)
					})

					It("returns Unsupported Media Type", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnsupportedMediaType))
					})

					It("does not save it", func() {
						Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
					})
				})

				Context("when the config contains extra keys at the toplevel", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/json")

						remoraPayload, err := json.Marshal(map[string]interface{}{
							"extra": "noooooo",

							"meta": map[string]interface{}{
								"whoa": "lol",
							},

							"jobs": []map[string]interface{}{
								{
									"name":   "some-job",
									"public": true,
									"plan":   []atc.Step{},
								},
							},
						})
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(remoraPayload)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("saves it", func() {
						Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

						name, savedConfig, id, initiallyPaused := dbTeam.SavePipelineArgsForCall(0)
						Expect(name).To(Equal("a-pipeline"))
						Expect(savedConfig).To(Equal(atc.Config{
							Jobs: atc.JobConfigs{
								{
									Name:         "some-job",
									Public:       true,
									PlanSequence: []atc.Step{},
								},
							},
						}))
						Expect(id).To(Equal(db.ConfigVersion(42)))
						Expect(initiallyPaused).To(BeTrue())
					})
				})

				Context("when the config contains extra keys nested under a valid key", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/json")

						remoraPayload, err := json.Marshal(map[string]interface{}{
							"extra": "noooooo",

							"jobs": []map[string]interface{}{
								{
									"name":  "some-job",
									"pubic": true,
									"plan":  []atc.Step{},
								},
							},
						})
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(remoraPayload)
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("returns an error in the response body", func() {
						Expect(ioutil.ReadAll(response.Body)).To(ContainSubstring(`malformed config: error unmarshaling JSON: while decoding JSON: json: unknown field \"pubic\"`))
					})

					It("does not save it", func() {
						Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
					})
				})
			})

			Context("when a config version is malformed", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigVersionHeader, "forty-two")
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns an error in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
									"config version is malformed: expected integer"
								]
							}`))
				})

				It("does not save it", func() {
					Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not save the config", func() {
				Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
			})
		})
	})
})
