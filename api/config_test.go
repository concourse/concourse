package api_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"gopkg.in/yaml.v2"
)

type RemoraConfig struct {
	atc.Config

	Extra string `json:"extra"`
}

var _ = Describe("Config API", func() {
	var (
		config atc.Config
	)

	BeforeEach(func() {
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

					InputConfigs: []atc.JobInputConfig{
						{
							RawName:  "some-input",
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							Passed: []string{"job-1", "job-2"},
						},
					},

					OutputConfigs: []atc.JobOutputConfig{
						{
							Resource: "some-resource",
							Params: atc.Params{
								"some-param": "some-value",
							},
							RawPerformOn: []atc.Condition{"success", "failure"},
						},
					},
				},
			},
		}
	})

	Describe("GET /api/v1/config", func() {
		var (
			response *http.Response
		)

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/config", nil)
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

				It("returns the config ID as X-Concourse-Config-ID", func() {
					Ω(response.Header.Get(atc.ConfigIDHeader)).Should(Equal("1"))
				})

				It("returns the config", func() {
					var returnedConfig atc.Config
					err := json.NewDecoder(response.Body).Decode(&returnedConfig)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(returnedConfig).Should(Equal(config))
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

	Describe("PUT /api/v1/config", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("PUT", server.URL+"/api/v1/config", nil)
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

			Context("when a config ID is specified", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigIDHeader, "42")
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

							config, id := configDB.SaveConfigArgsForCall(0)
							Ω(config).Should(Equal(config))
							Ω(id).Should(Equal(db.ConfigID(42)))
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(errors.New("oh no!"))
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

							config, id := configDB.SaveConfigArgsForCall(0)
							Ω(config).Should(Equal(config))
							Ω(id).Should(Equal(db.ConfigID(42)))
						})

						Context("when the payload contains suspicious types", func() {
							BeforeEach(func() {
								payload := []byte(`
jobs:
- name: some-job
  config:
    run:
      path: ls
    params:
      FOO: true
      BAR: 1
      BAZ: 1.9
`)

								request.Body = gbytes.BufferWithBytes(payload)
							})

							It("returns 200", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusOK))
							})

							It("saves it", func() {
								Ω(configDB.SaveConfigCallCount()).Should(Equal(1))

								config, id := configDB.SaveConfigArgsForCall(0)
								Ω(config).Should(Equal(atc.Config{
									Jobs: atc.JobConfigs{
										{
											Name: "some-job",
											TaskConfig: &atc.TaskConfig{
												Run: atc.BuildRunConfig{
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
								Ω(id).Should(Equal(db.ConfigID(42)))
							})
						})

						Context("and saving it fails", func() {
							BeforeEach(func() {
								configDB.SaveConfigReturns(errors.New("oh no!"))
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

			Context("when a config ID is not specified", func() {
				BeforeEach(func() {
					// don't
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns an error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("no config ID specified")))
				})

				It("does not save it", func() {
					Ω(configDB.SaveConfigCallCount()).Should(BeZero())
				})
			})

			Context("when a config ID is malformed", func() {
				BeforeEach(func() {
					request.Header.Set(atc.ConfigIDHeader, "forty-two")
				})

				It("returns 400", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusBadRequest))
				})

				It("returns an error in the response body", func() {
					Ω(ioutil.ReadAll(response.Body)).Should(Equal([]byte("config ID is malformed: expected integer")))
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
