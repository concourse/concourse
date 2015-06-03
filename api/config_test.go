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

			Jobs: atc.JobConfigs{
				{
					Name: "some-job",

					Public: true,

					TaskConfigPath: "some/config/path.yml",
					TaskConfig: &atc.TaskConfig{
						Image: "some-image",
					},

					Privileged: true,

					Serial: true,

					Plan: atc.PlanSequence{
						{
							Params: atc.Params{
								"some-param": "some-value",
								"nested": map[string]interface{}{
									"key": "value",
									"nested": map[string]interface{}{
										"key": "value",
									},
								},
							},
						},
					},

					InputConfigs: []atc.JobInputConfig{
						{
							RawName:  "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
								"nested": map[string]interface{}{
									"key": "value",
									"nested": map[string]interface{}{
										"key": "value",
									},
								},
							},
							Passed: []string{"job-1", "job-2"},
						},
					},

					OutputConfigs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
								"nested": map[string]interface{}{
									"key": "value",
									"nested": map[string]interface{}{
										"key": "value",
									},
								},
							},
							RawPerformOn: []atc.Condition{"success", "failure"},
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
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
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
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns the config version as X-Concourse-Config-Version", func() {
					Ω(response.Header.Get(atc.ConfigVersionHeader)).Should(Equal("1"))
				})

				It("returns the config", func() {
					var returnedConfig atc.Config
					err := json.NewDecoder(response.Body).Decode(&returnedConfig)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returnedConfig).Should(Equal(config))
				})

				It("calls get config with the correct arguments", func() {
					name := configDB.GetConfigArgsForCall(0)
					Ω(name).Should(Equal("something-else"))
				})
			})

			Context("when getting the config fails", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
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
			Ω(err).ShouldNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when a config version is specified", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigVersionHeader, "42")
				})

				Context("when the config is valid", func() {
					Context("JSON", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/json")

							payload, err := json.Marshal(config)
							Ω(err).ShouldNot(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 200", func() {
							Ω(response.StatusCode).Should(Equal(http.StatusOK))
						})

						It("saves it", func() {
							Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

							name, config, id, pipelineState := configDB.SaveConfigArgsForCall(0)
							Ω(name).Should(Equal("a-pipeline"))
							Ω(config).Should(Equal(config))
							Ω(id).Should(Equal(db.ConfigVersion(42)))
							Ω(pipelineState).Should(Equal(db.PipelineNoChange))
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
							})

							It("returns the error in the response body", func() {
								Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("failed to save config: oh no!")))
							})
						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(true, nil)
							})

							It("returns 201", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusCreated))
							})
						})

						Context("when the config is invalid", func() {
							BeforeEach(func() {
								configValidationErr = errors.New("totally invalid")
							})

							It("returns 400", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
							})

							It("returns the validation error in the response body", func() {
								Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("totally invalid")))
							})

							It("does not save it", func() {
								Ω(configDB.SaveConfigCallCount()).Should(BeZero())
							})
						})
					})

					Context("YAML", func() {
						BeforeEach(func() {
							request.Header.Set("Content-Type", "application/x-yaml")

							payload, err := yaml.Marshal(config)
							Ω(err).ShouldNot(HaveOccurred())

							request.Body = gbytes.BufferWithBytes(payload)
						})

						It("returns 200", func() {
							Ω(response.StatusCode).Should(Equal(http.StatusOK))
						})

						It("saves it", func() {
							Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

							name, config, id, pipelineState := configDB.SaveConfigArgsForCall(0)
							Ω(name).Should(Equal("a-pipeline"))
							Ω(config).Should(Equal(config))
							Ω(id).Should(Equal(db.ConfigVersion(42)))
							Ω(pipelineState).Should(Equal(db.PipelineNoChange))
						})

						It("does not give the DB a map of empty interfaces to empty interfaces", func() {
							Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

							_, config, _, _ := configDB.SaveConfigArgsForCall(0)
							Ω(config).Should(Equal(config))

							_, err := json.Marshal(config)
							Ω(err).ShouldNot(HaveOccurred())
						})

						Context("when the payload contains suspicious types", func() {
							BeforeEach(func() {
								payload := `
jobs:
- name: some-job
  config:
    run:
      path: ls
    params:
      FOO: true
      BAR: 1
      BAZ: 1.9`

								request.Body = ioutil.NopCloser(bytes.NewBufferString(payload))
							})

							It("returns 200", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusOK))
							})

							It("saves it", func() {
								Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

								name, config, id, pipelineState := configDB.SaveConfigArgsForCall(0)
								Ω(name).Should(Equal("a-pipeline"))
								Ω(config).Should(Equal(atc.Config{
									Jobs: atc.JobConfigs{
										{
											Name: "some-job",
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
								}))
								Ω(id).Should(Equal(db.ConfigVersion(42)))
								Ω(pipelineState).Should(Equal(db.PipelineNoChange))
							})
						})

						Context("when it's the first time the pipeline has been created", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(true, nil)
							})

							It("returns 201", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusCreated))
							})
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
							})

							It("returns the error in the response body", func() {
								Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("failed to save config: oh no!")))
							})
						})

						Context("when the config is invalid", func() {
							BeforeEach(func() {
								configValidationErr = errors.New("totally invalid")
							})

							It("returns 400", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
							})

							It("returns the validation error in the response body", func() {
								Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("totally invalid")))
							})

							It("does not save it", func() {
								Ω(configDB.SaveConfigCallCount()).Should(BeZero())
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
								Ω(err).ShouldNot(HaveOccurred())

								yml, err := yaml.Marshal(config)
								Ω(err).ShouldNot(HaveOccurred())

								_, err = yamlWriter.Write(yml)

								Ω(err).ShouldNot(HaveOccurred())

								if pausedValue != "" {
									err = writer.WriteField("paused", pausedValue)
									Ω(err).ShouldNot(HaveOccurred())
								}

								writer.Close()

								request.Header.Set("Content-Type", writer.FormDataContentType())
								request.Body = gbytes.BufferWithBytes(body.Bytes())
							})

							It("returns 200", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusOK))
							})

							It("saves it", func() {
								Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

								name, config, id, pipelineState := configDB.SaveConfigArgsForCall(0)
								Ω(name).Should(Equal("a-pipeline"))
								Ω(config).Should(Equal(config))
								Ω(id).Should(Equal(db.ConfigVersion(42)))
								Ω(pipelineState).Should(Equal(expectedDBValue))
							})

							Context("when it's the first time the pipeline has been created", func() {
								BeforeEach(func() {
									configDB.SaveConfigReturns(true, nil)
								})

								It("returns 201", func() {
									Ω(response.StatusCode).Should(Equal(http.StatusCreated))
								})
							})

							Context("and saving it fails", func() {
								BeforeEach(func() {
									configDB.SaveConfigReturns(false, errors.New("oh no!"))
								})

								It("returns 500", func() {
									Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
								})

								It("returns the error in the response body", func() {
									Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("failed to save config: oh no!")))
								})
							})

							Context("when the config is invalid", func() {
								BeforeEach(func() {
									configValidationErr = errors.New("totally invalid")
								})

								It("returns 400", func() {
									Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
								})

								It("returns the validation error in the response body", func() {
									Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("totally invalid")))
								})

								It("does not save it", func() {
									Ω(configDB.SaveConfigCallCount()).Should(BeZero())
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
					})
				})

				Context("when the Content-Type is unsupported", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/x-toml")

						payload, err := yaml.Marshal(config)
						Ω(err).ShouldNot(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(payload)
					})

					It("returns Unsupported Media Type", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusUnsupportedMediaType))
					})

					It("does not save it", func() {
						Ω(configDB.SaveConfigCallCount()).Should(BeZero())
					})
				})

				Context("when the config contains extra keys", func() {
					BeforeEach(func() {
						request.Header.Set("Content-Type", "application/json")

						remoraPayload, err := json.Marshal(RemoraConfig{
							Config: config,
							Extra:  "noooooo",
						})
						Ω(err).ShouldNot(HaveOccurred())

						request.Body = gbytes.BufferWithBytes(remoraPayload)
					})

					It("returns 400", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
					})

					It("returns an error in the response body", func() {
						body, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(body).Should(ContainSubstring("unknown/extra keys:"))
						Ω(body).Should(ContainSubstring("- extra"))
					})

					It("does not save it", func() {
						Ω(configDB.SaveConfigCallCount()).Should(BeZero())
					})
				})
			})

			Context("when a config version is not specified", func() {
				BeforeEach(func() {
					// don't
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns an error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("no config version specified")))
				})

				It("does not save it", func() {
					Ω(configDB.SaveConfigCallCount()).Should(BeZero())
				})
			})

			Context("when a config version is malformed", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigVersionHeader, "forty-two")
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns an error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("config version is malformed: expected integer")))
				})

				It("does not save it", func() {
					Ω(configDB.SaveConfigCallCount()).Should(BeZero())
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})

			It("does not save the config", func() {
				Ω(configDB.SaveConfigCallCount()).Should(BeZero())
			})

			It("returns the error in the response body", func() {
				Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("not authorized")))
			})
		})
	})
})
