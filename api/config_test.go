package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/creds/credsfakes"
	"github.com/concourse/atc/creds/noop"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config API", func() {
	var (
		pipelineConfig   atc.Config
		requestGenerator *rata.RequestGenerator
		fakeaccess       *accessorfakes.FakeAccess
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)

		fakeaccess = new(accessorfakes.FakeAccess)

		pipelineConfig = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"some-job"},
					Resources: []string{"some-resource"},
				},
			},

			Resources: atc.ResourceConfigs{
				{
					Name: "some-resource",
					Type: "some-type",
					Source: atc.Source{
						"source-config": "some-value",
						"nested": map[string]interface{}{
							"key": "value",
							"array": []interface{}{
								map[string]interface{}{
									"key": "value",
								},
							},
						},
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
					Plan: atc.PlanSequence{
						{
							Get:      "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
								"nested": map[string]interface{}{
									"key": "value",
									"array": []interface{}{
										map[string]interface{}{
											"key": "value",
										},
									},
								},
							},
						},
						{
							Task:       "some-task",
							Privileged: true,
							TaskConfig: &atc.TaskConfig{
								Platform:  "linux",
								RootfsURI: "some-image",
								Run: atc.TaskRunConfig{
									Path: "/path/to/run",
								},
							},
						},
						{
							Put:      "some-output",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
								"nested": map[string]interface{}{
									"key": "value",
									"array": []interface{}{
										map[string]interface{}{
											"key": "value",
										},
									},
								},
							},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
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
						fakeTeam.PipelineReturns(fakePipeline, true, nil)
					})

					Context("when the jobs are found", func() {
						var fakeJob *dbfakes.FakeJob
						BeforeEach(func() {
							fakeJob = new(dbfakes.FakeJob)
							fakeJob.ConfigReturns(atc.JobConfig{
								Name:   "some-job",
								Public: true,
								Serial: true,
								Plan: atc.PlanSequence{
									{
										Get:      "some-input",
										Resource: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
											"nested": map[string]interface{}{
												"key": "value",
												"array": []interface{}{
													map[string]interface{}{
														"key": "value",
													},
												},
											},
										},
									},
									{
										Task:       "some-task",
										Privileged: true,
										TaskConfig: &atc.TaskConfig{
											Platform:  "linux",
											RootfsURI: "some-image",
											Run: atc.TaskRunConfig{
												Path: "/path/to/run",
											},
										},
									},
									{
										Put:      "some-output",
										Resource: "some-resource",
										Params: atc.Params{
											"some-param": "some-value",
											"nested": map[string]interface{}{
												"key": "value",
												"array": []interface{}{
													map[string]interface{}{
														"key": "value",
													},
												},
											},
										},
									},
								},
							})

							fakePipeline.JobsReturns(db.Jobs{fakeJob}, nil)
						})

						Context("when the resources are found", func() {
							var fakeResource *dbfakes.FakeResource
							BeforeEach(func() {
								fakeResource = new(dbfakes.FakeResource)
								fakeResource.NameReturns("some-resource")
								fakeResource.TypeReturns("some-type")
								fakeResource.SourceReturns(atc.Source{
									"source-config": "some-value",
									"nested": map[string]interface{}{
										"key": "value",
										"array": []interface{}{
											map[string]interface{}{
												"key": "value",
											},
										},
									},
								})

								fakePipeline.ResourcesReturns(db.Resources{fakeResource}, nil)
							})

							Context("when the resource types are found", func() {
								var fakeResourceType *dbfakes.FakeResourceType
								BeforeEach(func() {
									fakeResourceType = new(dbfakes.FakeResourceType)
									fakeResourceType.NameReturns("custom-resource")
									fakeResourceType.TypeReturns("custom-type")
									fakeResourceType.SourceReturns(atc.Source{"custom": "source"})
									fakeResourceType.TagsReturns(atc.Tags{"some-tag"})

									fakePipeline.ResourceTypesReturns(db.ResourceTypes{fakeResourceType}, nil)
								})

								It("returns 200", func() {
									Expect(response.StatusCode).To(Equal(http.StatusOK))
								})

								It("returns Content-Type 'application/json'", func() {
									Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
								})

								It("returns the config version as X-Concourse-Config-Version", func() {
									Expect(response.Header.Get(atc.ConfigVersionHeader)).To(Equal("1"))
								})

								It("returns the config", func() {
									var actualConfigResponse atc.ConfigResponse
									err := json.NewDecoder(response.Body).Decode(&actualConfigResponse)
									Expect(err).NotTo(HaveOccurred())

									rawConfig, err := json.Marshal(pipelineConfig)
									Expect(err).NotTo(HaveOccurred())

									Expect(actualConfigResponse).To(Equal(atc.ConfigResponse{
										Config:    &pipelineConfig,
										RawConfig: atc.RawConfig(rawConfig),
									}))
								})
							})

							Context("when finding the resource types fails", func() {
								BeforeEach(func() {
									fakePipeline.ResourceTypesReturns(nil, errors.New("failed"))
								})

								It("returns 500", func() {
									Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})
						})

						Context("when finding the resources fails", func() {
							BeforeEach(func() {
								fakePipeline.ResourcesReturns(nil, errors.New("failed"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when finding the jobs fails", func() {
						BeforeEach(func() {
							fakePipeline.JobsReturns(nil, errors.New("failed"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
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
				fakeaccess.IsAuthenticatedReturns(false)
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
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
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})

						It("returns error JSON", func() {
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"malformed config"
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
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})

						It("returns error JSON", func() {
							Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
								{
									"errors": [
										"malformed config"
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

						It("returns Content-Type 'application/json'", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})

						It("saves it", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(db.PipelineNoChange))
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
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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

						It("returns Content-Type 'application/json'", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})

						It("saves it", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(db.PipelineNoChange))
						})

						It("does not give the DB a map of empty interfaces to empty interfaces", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							_, savedConfig, _, _ := dbTeam.SavePipelineArgsForCall(0)
							Expect(savedConfig).To(Equal(pipelineConfig))

							_, err := json.Marshal(pipelineConfig)
							Expect(err).NotTo(HaveOccurred())
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
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
							})

							It("saves it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

								name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
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
											Plan: atc.PlanSequence{
												{
													Get: "some-resource",
												},
												{
													Task: "some-task",
													TaskConfig: &atc.TaskConfig{
														Platform: "linux",

														Run: atc.TaskRunConfig{
															Path: "ls",
														},

														Params: map[string]string{
															"FOO": "true",
															"BAR": "1",
															"BAZ": "1.9",
														},
													},
												},
											},
										},
									},
								}))

								Expect(id).To(Equal(db.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(db.PipelineNoChange))
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
											Plan: atc.PlanSequence{
												{
													Get: "some-resource",
												},
												{
													Task: "some-task",
													TaskConfig: &atc.TaskConfig{
														Platform: "linux",

														Run: atc.TaskRunConfig{
															Path: "ls",
														},

														Params: map[string]string{
															"FOO": "((BAR))",
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
										fakeVariables := new(credsfakes.FakeVariables)
										fakeVariablesFactory.NewVariablesReturns(fakeVariables)
										fakeVariables.GetReturns("this-string-value-doesn't-matter", true, nil)
									})

									It("passes validation and saves it un-interpolated", func() {
										Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

										name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
										Expect(name).To(Equal("a-pipeline"))
										Expect(savedConfig).To(Equal(payloadAsConfig))

										Expect(id).To(Equal(db.ConfigVersion(42)))
										Expect(pipelineState).To(Equal(db.PipelineNoChange))
									})

									It("returns 200", func() {
										Expect(response.StatusCode).To(Equal(http.StatusOK))
									})
								})

								Context("when the credential does not exist in the credential manager", func() {
									BeforeEach(func() {
										fakeVariables := new(credsfakes.FakeVariables)
										fakeVariablesFactory.NewVariablesReturns(fakeVariables)
										fakeVariables.GetReturns(nil, false, nil) // nil value, not found, no error
									})

									It("returns 400", func() {
										Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
									})

									It("returns the credential name that was missing", func() {
										Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"errors":["1 error occurred:\n\n* Expected to find variables: BAR"]}`))
									})
								})

								Context("when a credentials manager is not used", func() {
									BeforeEach(func() {
										fakeVariables := noop.Noop{}
										fakeVariablesFactory.NewVariablesReturns(&fakeVariables)
									})

									It("returns 400", func() {
										Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
									})

									It("returns the credential name that was missing", func() {
										Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{"errors":["1 error occurred:\n\n* Expected to find variables: BAR"]}`))
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
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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

					Context("multi-part requests", func() {
						var pausedValue string
						var expectedDBValue db.PipelinePausedState

						writeMultiPart := func() {
							body := &bytes.Buffer{}
							writer := multipart.NewWriter(body)

							yamlWriter, err := writer.CreatePart(
								textproto.MIMEHeader{
									"Content-type": {"application/x-yaml"},
								},
							)
							Expect(err).NotTo(HaveOccurred())

							yml, err := yaml.Marshal(pipelineConfig)
							Expect(err).NotTo(HaveOccurred())

							_, err = yamlWriter.Write(yml)

							Expect(err).NotTo(HaveOccurred())

							if pausedValue != "" {
								err = writer.WriteField("paused", pausedValue)
								Expect(err).NotTo(HaveOccurred())
							}

							_ = writer.Close()

							request.Header.Set("Content-Type", writer.FormDataContentType())
							request.Body = gbytes.BufferWithBytes(body.Bytes())
						}

						itSavesThePipeline := func() {
							BeforeEach(writeMultiPart)

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("returns Content-Type 'application/json'", func() {
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
							})

							It("saves it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

								name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
								Expect(name).To(Equal("a-pipeline"))
								Expect(savedConfig).To(Equal(pipelineConfig))
								Expect(id).To(Equal(db.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(expectedDBValue))
							})

							Context("when it's the first time the pipeline has been created", func() {
								BeforeEach(func() {
									returnedPipeline := new(dbfakes.FakePipeline)
									dbTeam.SavePipelineReturns(returnedPipeline, true, nil)
								})

								It("returns 201", func() {
									Expect(response.StatusCode).To(Equal(http.StatusCreated))
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
									writeMultiPart()
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
								})

								It("returns Content-Type 'application/json'", func() {
									Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
								})

								It("returns error JSON", func() {
									Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
										"errors": [
											"invalid groups:\n\tgroup 'some-group' has unknown resource 'missing-resource'\n"
										]
									}`))
								})

								It("does not save it", func() {
									Expect(dbTeam.SavePipelineCallCount()).To(BeZero())
								})
							})

							Context("when the config includes deprecations", func() {
								BeforeEach(func() {
									pipelineConfig.Jobs[0].Plan[1].ImageArtifactName = "some-image-artifact"
									writeMultiPart()
								})

								It("returns warnings", func() {
									Expect(response.StatusCode).To(Equal(http.StatusOK))
									Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
										"warnings": [{
											"type": "pipeline",
											"message": "jobs.some-job.plan[1].task.some-task specifies an image artifact to use as the container's image but also specifies an image or image resource in the task configuration; the image artifact takes precedence"
										}]
									}`))
								})
							})
						}

						Context("when paused is specified", func() {
							BeforeEach(func() {
								pausedValue = "true"
								expectedDBValue = db.PipelinePaused
							})

							itSavesThePipeline()
						})

						Context("when unpaused is specified", func() {
							BeforeEach(func() {
								pausedValue = "false"
								expectedDBValue = db.PipelineUnpaused
							})

							itSavesThePipeline()
						})

						Context("when neither paused or unpaused is specified", func() {
							BeforeEach(func() {
								pausedValue = ""
								expectedDBValue = db.PipelineNoChange
							})

							itSavesThePipeline()
						})

						Context("when a strange paused value is specified", func() {
							BeforeEach(func() {
								body := &bytes.Buffer{}
								writer := multipart.NewWriter(body)

								yamlWriter, err := writer.CreatePart(
									textproto.MIMEHeader{
										"Content-type": {"application/x-yaml"},
									},
								)
								Expect(err).NotTo(HaveOccurred())

								yml, err := yaml.Marshal(pipelineConfig)
								Expect(err).NotTo(HaveOccurred())

								_, err = yamlWriter.Write(yml)

								Expect(err).NotTo(HaveOccurred())

								err = writer.WriteField("paused", "junk")
								Expect(err).NotTo(HaveOccurred())

								_ = writer.Close()

								request.Header.Set("Content-Type", writer.FormDataContentType())
								request.Body = gbytes.BufferWithBytes(body.Bytes())
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})

							It("returns Content-Type 'application/json'", func() {
								Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
							})

							It("returns error JSON", func() {
								Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
										"errors": [
											"invalid paused value"
										]
									}`))
							})
						})

						Context("when the config is malformed", func() {
							Context("JSON", func() {
								BeforeEach(func() {
									body := &bytes.Buffer{}
									writer := multipart.NewWriter(body)

									yamlWriter, err := writer.CreatePart(
										textproto.MIMEHeader{
											"Content-type": {"application/json"},
										},
									)
									Expect(err).NotTo(HaveOccurred())

									_, err = yamlWriter.Write([]byte("{"))

									Expect(err).NotTo(HaveOccurred())

									_ = writer.Close()

									request.Header.Set("Content-Type", writer.FormDataContentType())
									request.Body = gbytes.BufferWithBytes(body.Bytes())
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
								})

								It("returns Content-Type 'application/json'", func() {
									Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
								})

								It("returns error JSON", func() {
									Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
										{
											"errors": [
												"malformed config"
											]
										}`))
								})

								It("does not save anything", func() {
									Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
								})
							})

							Context("YAML", func() {
								BeforeEach(func() {
									body := &bytes.Buffer{}
									writer := multipart.NewWriter(body)

									yamlWriter, err := writer.CreatePart(
										textproto.MIMEHeader{
											"Content-type": {"application/x-yaml"},
										},
									)
									Expect(err).NotTo(HaveOccurred())

									_, err = yamlWriter.Write([]byte("{"))
									Expect(err).NotTo(HaveOccurred())

									_ = writer.Close()

									request.Header.Set("Content-Type", writer.FormDataContentType())
									request.Body = gbytes.BufferWithBytes(body.Bytes())
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
								})

								It("returns Content-Type 'application/json'", func() {
									Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
								})

								It("returns error JSON", func() {
									Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
										{
											"errors": [
												"malformed config"
											]
										}`))
								})

								It("does not save anything", func() {
									Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
								})
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
									"plan":   atc.PlanSequence{},
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
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("saves it", func() {
						Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

						name, savedConfig, id, _ := dbTeam.SavePipelineArgsForCall(0)
						Expect(name).To(Equal("a-pipeline"))
						Expect(savedConfig).To(Equal(atc.Config{
							Jobs: atc.JobConfigs{
								{
									Name:   "some-job",
									Public: true,
									Plan:   atc.PlanSequence{},
								},
							},
						}))
						Expect(id).To(Equal(db.ConfigVersion(42)))
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
									"plan":  atc.PlanSequence{},
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
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
					})

					It("returns an error in the response body", func() {
						Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
									"unknown/extra keys:\n  - jobs[0].pubic\n"
								]
							}`))
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
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				fakeaccess.IsAuthenticatedReturns(false)
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
