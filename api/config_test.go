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
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type RemoraConfig struct {
	atc.Config

	Extra string `json:"extra"`
}

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
								Image: "some-image",
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
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", true, true)
			})

			Context("when the config can be loaded", func() {
				BeforeEach(func() {
					teamDB.GetConfigReturns(pipelineConfig, atc.RawConfig("raw-config"), 1, nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the config version as X-Concourse-Config-Version", func() {
					Expect(response.Header.Get(atc.ConfigVersionHeader)).To(Equal("1"))
				})

				It("returns the config", func() {
					var actualConfigResponse atc.ConfigResponse
					err := json.NewDecoder(response.Body).Decode(&actualConfigResponse)
					Expect(err).NotTo(HaveOccurred())

					Expect(actualConfigResponse).To(Equal(atc.ConfigResponse{
						Config:    &pipelineConfig,
						RawConfig: atc.RawConfig("raw-config"),
					}))
				})

				It("calls get config with the correct arguments", func() {
					Expect(teamDB.GetConfigArgsForCall(0)).To(Equal("something-else"))
				})
			})

			Context("when getting the config fails", func() {
				BeforeEach(func() {
					teamDB.GetConfigReturns(atc.Config{}, atc.RawConfig(""), 0, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when getting the config fails because it is malformed", func() {
				BeforeEach(func() {
					teamDB.GetConfigReturns(atc.Config{}, atc.RawConfig("raw-config"), 42, atc.MalformedConfigError{UnmarshalError: errors.New("invalid character")})
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns error JSON", func() {
					Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
					{
						"config": null,
						"errors": [
						  "malformed config: invalid character"
						],
						"raw_config": "raw-config"
					}`))
				})

				It("returns the config version header", func() {
					Expect(response.Header.Get(atc.ConfigVersionHeader)).To(Equal("42"))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
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
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("a-team", true, true)
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

						It("saves it", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(dbng.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(dbng.PipelineNoChange))
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
								returnedPipeline := new(dbngfakes.FakePipeline)
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

						It("saves it", func() {
							Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

							name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(pipelineConfig))
							Expect(id).To(Equal(dbng.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(dbng.PipelineNoChange))
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
jobs:
- name: some-job
  plan:
  - get: some-resource
  - task: some-task
    config:
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

							It("saves it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

								name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
								Expect(name).To(Equal("a-pipeline"))
								Expect(savedConfig).To(Equal(atc.Config{
									Resources: []atc.ResourceConfig{
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

								Expect(id).To(Equal(dbng.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(dbng.PipelineNoChange))
							})
						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								returnedPipeline := new(dbngfakes.FakePipeline)
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
						var expectedDBValue dbng.PipelinePausedState

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

							writer.Close()

							request.Header.Set("Content-Type", writer.FormDataContentType())
							request.Body = gbytes.BufferWithBytes(body.Bytes())
						}

						itSavesThePipeline := func() {
							BeforeEach(writeMultiPart)

							It("returns 200", func() {
								Expect(response.StatusCode).To(Equal(http.StatusOK))
							})

							It("saves it", func() {
								Expect(dbTeam.SavePipelineCallCount()).To(Equal(1))

								name, savedConfig, id, pipelineState := dbTeam.SavePipelineArgsForCall(0)
								Expect(name).To(Equal("a-pipeline"))
								Expect(savedConfig).To(Equal(pipelineConfig))
								Expect(id).To(Equal(dbng.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(expectedDBValue))
							})

							Context("when it's the first time the pipeline has been created", func() {
								BeforeEach(func() {
									returnedPipeline := new(dbngfakes.FakePipeline)
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
									pipelineConfig.Jobs[0].Plan[1].TaskConfigPath = "some/config/path.yml"
									writeMultiPart()
								})

								It("returns warnings", func() {
									Expect(response.StatusCode).To(Equal(http.StatusOK))
									Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`{
										"warnings": [{
											"type": "deprecation",
											"message": "jobs.some-job.plan[1].task.some-task specifies both ` + "`file` and `config`" + ` in a task step"
										}]
									}`))
								})
							})
						}

						Context("when paused is specified", func() {
							BeforeEach(func() {
								pausedValue = "true"
								expectedDBValue = dbng.PipelinePaused
							})

							itSavesThePipeline()
						})

						Context("when unpaused is specified", func() {
							BeforeEach(func() {
								pausedValue = "false"
								expectedDBValue = dbng.PipelineUnpaused
							})

							itSavesThePipeline()
						})

						Context("when neither paused or unpaused is specified", func() {
							BeforeEach(func() {
								pausedValue = ""
								expectedDBValue = dbng.PipelineNoChange
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

								writer.Close()

								request.Header.Set("Content-Type", writer.FormDataContentType())
								request.Body = gbytes.BufferWithBytes(body.Bytes())
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
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

									writer.Close()

									request.Header.Set("Content-Type", writer.FormDataContentType())
									request.Body = gbytes.BufferWithBytes(body.Bytes())
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
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

									writer.Close()

									request.Header.Set("Content-Type", writer.FormDataContentType())
									request.Body = gbytes.BufferWithBytes(body.Bytes())
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
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

				Context("when the config contains extra keys", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/json")

						remoraPayload, err := json.Marshal(RemoraConfig{
							Config: pipelineConfig,
							Extra:  "noooooo",
						})
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(remoraPayload)
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})

					It("returns an error in the response body", func() {
						Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
									"unknown/extra keys:\n  - extra\n"
								]
							}`))
					})

					It("does not save it", func() {
						Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
					})
				})
			})

			Context("when a config version is not specified", func() {
				BeforeEach(func() {
					// don't
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
				})

				It("returns an error in the response body", func() {
					Expect(ioutil.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
									"no config version specified"
								]
							}`))
				})

				It("does not save it", func() {
					Expect(dbTeam.SavePipelineCallCount()).To(Equal(0))
				})
			})

			Context("when a config version is malformed", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigVersionHeader, "forty-two")
				})

				It("returns 400", func() {
					Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
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
				authValidator.IsAuthenticatedReturns(false)
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
