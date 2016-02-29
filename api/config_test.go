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
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/tedsuo/rata"
	"gopkg.in/yaml.v2"
)

type RemoraConfig struct {
	atc.Config

	Extra string `json:"extra"`
}

var _ = Describe("Config API", func() {
	var (
		config           atc.Config
		requestGenerator *rata.RequestGenerator
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)

		config = atc.Config{
			Groups: atc.GroupConfigs{
				{
					Name:      "some-group",
					Jobs:      []string{"job-1", "job-2"},
					Resources: []string{"resource-1", "resource-2"},
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
							"nested": map[string]interface{}{
								"key": "value",
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
							Passed:   []string{"job-1", "job-2"},
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
						{
							Task:           "some-task",
							Privileged:     true,
							TaskConfigPath: "some/config/path.yml",
							TaskConfig: &atc.TaskConfig{
								Image: "some-image",
							},
						},
						{
							Put:      "some-output",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
						},
					},
				},
			},
		}
	})

	Describe("GET /api/v1/pipelines/:name/config", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			req, err := requestGenerator.CreateRequest(atc.GetConfig, rata.Params{
				"pipeline_name": "something-else",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when the config can be loaded", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(config, 1, nil)
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns the config version as X-Concourse-Config-Version", func() {
					Expect(response.Header.Get(atc.ConfigVersionHeader)).To(Equal("1"))
				})

				It("returns the config", func() {
					var returnedConfig atc.Config
					err := json.NewDecoder(response.Body).Decode(&returnedConfig)
					Expect(err).NotTo(HaveOccurred())

					Expect(returnedConfig).To(Equal(config))
				})

				It("calls get config with the correct arguments", func() {
					teamName, name := configDB.GetConfigArgsForCall(0)
					Expect(teamName).To(Equal(atc.DefaultTeamName))
					Expect(name).To(Equal("something-else"))
				})
			})

			Context("when getting the config fails", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
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
		})
	})

	Describe("PUT /api/v1/pipelines/:name/config", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error
			request, err = requestGenerator.CreateRequest(atc.SaveConfig, rata.Params{
				"pipeline_name": "a-pipeline",
			}, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
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

						It("does not save anything", func() {
							Expect(configDB.SaveConfigCallCount()).To(Equal(0))
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

						It("does not save anything", func() {
							Expect(configDB.SaveConfigCallCount()).To(Equal(0))
						})
					})
				})

				Context("when the config is valid", func() {
					Context("JSON", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/json")

							payload, err := json.Marshal(config)
							Expect(err).NotTo(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 204", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNoContent))
						})

						It("saves it", func() {
							Expect(configDB.SaveConfigCallCount()).To(Equal(1))

							_, name, savedConfig, id, pipelineState := configDB.SaveConfigArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(config))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(db.PipelineNoChange))
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(db.SavedPipeline{}, false, errors.New("oh no!"))
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
								returnedPipeline := db.SavedPipeline{
									ID:     1234,
									Paused: true,
									TeamID: 1,
									Pipeline: db.Pipeline{
										Name:    "a-pipeline",
										Config:  config,
										Version: db.ConfigVersion(42),
									},
								}
								configDB.SaveConfigReturns(returnedPipeline, true, nil)
							})

							It("returns 201", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})
						})

						Context("when the config is invalid", func() {
							BeforeEach(func() {
								configValidationErr = errors.New("totally invalid")
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})

							It("returns the validation error in the response body", func() {
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("totally invalid")))
							})

							It("does not save it", func() {
								Expect(configDB.SaveConfigCallCount()).To(BeZero())
							})
						})
					})

					Context("YAML", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/x-yaml")

							payload, err := yaml.Marshal(config)
							Expect(err).NotTo(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 204", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNoContent))
						})

						It("saves it", func() {
							Expect(configDB.SaveConfigCallCount()).To(Equal(1))

							_, name, savedConfig, id, pipelineState := configDB.SaveConfigArgsForCall(0)
							Expect(name).To(Equal("a-pipeline"))
							Expect(savedConfig).To(Equal(config))
							Expect(id).To(Equal(db.ConfigVersion(42)))
							Expect(pipelineState).To(Equal(db.PipelineNoChange))
						})

						It("does not give the DB a map of empty interfaces to empty interfaces", func() {
							Expect(configDB.SaveConfigCallCount()).To(Equal(1))

							_, _, savedConfig, _, _ := configDB.SaveConfigArgsForCall(0)
							Expect(savedConfig).To(Equal(config))

							_, err := json.Marshal(config)
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

							It("returns 204", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNoContent))
							})

							It("saves it", func() {
								Expect(configDB.SaveConfigCallCount()).To(Equal(1))

								_, name, savedConfig, id, pipelineState := configDB.SaveConfigArgsForCall(0)
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

								Expect(id).To(Equal(db.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(db.PipelineNoChange))
							})
						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								returnedPipeline := db.SavedPipeline{
									ID:     1234,
									Paused: true,
									TeamID: 1,
									Pipeline: db.Pipeline{
										Name:    "a-pipeline",
										Config:  config,
										Version: db.ConfigVersion(42),
									},
								}
								configDB.SaveConfigReturns(returnedPipeline, true, nil)
							})

							It("returns 201", func() {
								Expect(response.StatusCode).To(Equal(http.StatusCreated))
							})
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(db.SavedPipeline{}, false, errors.New("oh no!"))
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
								configValidationErr = errors.New("totally invalid")
							})

							It("returns 400", func() {
								Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							})

							It("returns the validation error in the response body", func() {
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("totally invalid")))
							})

							It("does not save it", func() {
								Expect(configDB.SaveConfigCallCount()).To(BeZero())
							})
						})
					})

					Context("multi-part requests", func() {
						var pausedValue string
						var expectedDBValue db.PipelinePausedState

						itSavesThePipeline := func() {
							BeforeEach(func() {
								body := &bytes.Buffer{}
								writer := multipart.NewWriter(body)

								yamlWriter, err := writer.CreatePart(
									textproto.MIMEHeader{
										"Content-type": {"application/x-yaml"},
									},
								)
								Expect(err).NotTo(HaveOccurred())

								yml, err := yaml.Marshal(config)
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
							})

							It("returns 204", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNoContent))
							})

							It("saves it", func() {
								Expect(configDB.SaveConfigCallCount()).To(Equal(1))

								_, name, savedConfig, id, pipelineState := configDB.SaveConfigArgsForCall(0)
								Expect(name).To(Equal("a-pipeline"))
								Expect(savedConfig).To(Equal(config))
								Expect(id).To(Equal(db.ConfigVersion(42)))
								Expect(pipelineState).To(Equal(expectedDBValue))
							})

							Context("when it's the first time the pipeline has been created", func() {
								BeforeEach(func() {
									returnedPipeline := db.SavedPipeline{
										ID:     1234,
										Paused: true,
										TeamID: 1,
										Pipeline: db.Pipeline{
											Name:    "a-pipeline",
											Config:  config,
											Version: db.ConfigVersion(42),
										},
									}
									configDB.SaveConfigReturns(returnedPipeline, true, nil)
								})

								It("returns 201", func() {
									Expect(response.StatusCode).To(Equal(http.StatusCreated))
								})
							})

							Context("and saving it fails", func() {
								BeforeEach(func() {
									configDB.SaveConfigReturns(db.SavedPipeline{}, false, errors.New("oh no!"))
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
									configValidationErr = errors.New("totally invalid")
								})

								It("returns 400", func() {
									Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
								})

								It("returns the validation error in the response body", func() {
									Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("totally invalid")))
								})

								It("does not save it", func() {
									Expect(configDB.SaveConfigCallCount()).To(BeZero())
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

								yml, err := yaml.Marshal(config)
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

							It("returns the validation error in the response body", func() {
								Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("invalid paused value")))
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

								It("does not save anything", func() {
									Expect(configDB.SaveConfigCallCount()).To(Equal(0))
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

								It("does not save anything", func() {
									Expect(configDB.SaveConfigCallCount()).To(Equal(0))
								})
							})
						})
					})
				})

				Context("when the Content-Type is unsupported", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/x-toml")

						payload, err := yaml.Marshal(config)
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(payload)
					})

					It("returns Unsupported Media Type", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnsupportedMediaType))
					})

					It("does not save it", func() {
						Expect(configDB.SaveConfigCallCount()).To(BeZero())
					})
				})

				Context("when the config contains extra keys", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/json")

						remoraPayload, err := json.Marshal(RemoraConfig{
							Config: config,
							Extra:  "noooooo",
						})
						Expect(err).NotTo(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(remoraPayload)
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
					})

					It("returns an error in the response body", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(ContainSubstring("unknown/extra keys:"))
						Expect(body).To(ContainSubstring("- extra"))
					})

					It("does not save it", func() {
						Expect(configDB.SaveConfigCallCount()).To(BeZero())
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
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("no config version specified")))
				})

				It("does not save it", func() {
					Expect(configDB.SaveConfigCallCount()).To(BeZero())
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
					Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("config version is malformed: expected integer")))
				})

				It("does not save it", func() {
					Expect(configDB.SaveConfigCallCount()).To(BeZero())
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
				Expect(configDB.SaveConfigCallCount()).To(BeZero())
			})
		})
	})
})
