package concourse_test

import (
	"io/ioutil"
	"net/http"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Configs", func() {
	Describe("PipelineConfig", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/config"

		Context("ATC returns the correct response when it exists", func() {
			var (
				expectedConfig    atc.Config
				expectedRawConfig atc.RawConfig
				expectedVersion   string
			)

			BeforeEach(func() {
				expectedConfig = atc.Config{
					Groups: atc.GroupConfigs{
						{
							Name:      "some-group",
							Jobs:      []string{"job-1", "job-2"},
							Resources: []string{"resource-1", "resource-2"},
						},
						{
							Name:      "some-other-group",
							Jobs:      []string{"job-3", "job-4"},
							Resources: []string{"resource-6", "resource-4"},
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
						{
							Name: "some-other-resource",
							Type: "some-other-type",
							Source: atc.Source{
								"source-config": "some-value",
								"FOO":           "((BAR))",
							},
						},
					},

					Jobs: atc.JobConfigs{
						{
							Name:   "some-job",
							Public: true,
							Serial: true,
						},
						{
							Name: "some-other-job",
						},
					},
				}

				expectedVersion = "42"

				expectedRawConfig = atc.RawConfig("raw-config")

				configResponse := atc.ConfigResponse{
					Config:    &expectedConfig,
					RawConfig: expectedRawConfig,
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, configResponse, http.Header{atc.ConfigVersionHeader: {expectedVersion}}),
					),
				)
			})

			It("returns the given config and version for that pipeline", func() {
				pipelineConfig, rawConfig, version, found, err := team.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelineConfig).To(Equal(expectedConfig))
				Expect(rawConfig).To(Equal(expectedRawConfig))
				Expect(version).To(Equal(expectedVersion))
				Expect(found).To(BeTrue())
			})
		})

		Context("when atc returns error messages", func() {
			BeforeEach(func() {
				configResponse := atc.ConfigResponse{Errors: []string{"config-error"}, RawConfig: atc.RawConfig("raw-config")}
				headers := http.Header{atc.ConfigVersionHeader: {"42"}}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, configResponse, headers),
					),
				)
			})

			It("returns an error", func() {
				_, actualRawConfig, actualConfigVersion, found, err := team.PipelineConfig("mypipeline")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("config-error"))
				Expect(actualRawConfig).To(Equal(atc.RawConfig("raw-config")))
				Expect(actualConfigVersion).To(Equal("42"))
				Expect(found).To(BeFalse())
			})
		})

		Context("when pipeline does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, _, _, found, err := team.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("ATC returns an error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns the error", func() {
				_, _, _, _, err := team.PipelineConfig("mypipeline")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("ATC does not return config version", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.ConfigResponse{Config: &atc.Config{}}),
					),
				)
			})

			It("returns an error", func() {
				_, _, _, _, err := team.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("CreateOrUpdatePipelineConfig", func() {
		var (
			expectedPipelineName string
			expectedVersion      string
			expectedConfig       []byte

			returnHeader int
			returnBody   []byte

			checkCredentials bool
		)

		BeforeEach(func() {
			expectedPipelineName = "mypipeline"
			expectedVersion = "42"
			expectedConfig = []byte("")

			expectedPath := "/api/v1/teams/some-team/pipelines/mypipeline/config"

			checkCredentials = false

			atcServer.RouteToHandler("PUT", expectedPath,
				ghttp.CombineHandlers(
					ghttp.VerifyHeaderKV(atc.ConfigVersionHeader, "42"),
					func(w http.ResponseWriter, r *http.Request) {
						defer r.Body.Close()
						bodyConfig, err := ioutil.ReadAll(r.Body)
						Expect(err).NotTo(HaveOccurred())

						receivedConfig := []byte("")

						err = yaml.Unmarshal(bodyConfig, &receivedConfig)
						Expect(err).NotTo(HaveOccurred())

						Expect(receivedConfig).To(Equal(expectedConfig))

						w.WriteHeader(returnHeader)
						w.Write(returnBody)
					},
				),
			)
		})

		Context("when creating a new config", func() {
			BeforeEach(func() {
				returnHeader = http.StatusCreated
				returnBody = []byte(`{"warnings":[
				  {"type": "warning-1-type", "message": "fake-warning1"},
					{"type": "warning-2-type", "message": "fake-warning2"}
				]}`)
			})

			It("returns true for created and false for updated", func() {
				created, updated, warnings, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(updated).To(BeFalse())
				Expect(warnings).To(ConsistOf([]concourse.ConfigWarning{
					{
						Type:    "warning-2-type",
						Message: "fake-warning2",
					},
					{
						Type:    "warning-1-type",
						Message: "fake-warning1",
					},
				}))
			})

			Context("when response contains bad JSON", func() {
				BeforeEach(func() {
					returnBody = []byte(`bad-json`)
				})

				It("returns an error", func() {
					_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when credential verification is enabled", func() {
				BeforeEach(func() {
					checkCredentials = true
				})

				Context("when credential doesn't exist in credentials manager", func() {
					BeforeEach(func() {
						returnHeader = http.StatusBadRequest
						returnBody = []byte(`{"errors":["Expected to find variables: BAR"]}`)
					})

					It("returns an error", func() {
						_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Expected to find variables: BAR"))
					})
				})

			})
		})

		Context("when updating a config", func() {
			BeforeEach(func() {
				returnHeader = http.StatusOK
				returnBody = []byte(`{"warnings":[
				  {"type": "warning-1-type", "message": "fake-warning1"},
					{"type": "warning-2-type", "message": "fake-warning2"}
				]}`)
			})

			It("returns false for created and true for updated", func() {
				created, updated, warnings, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeFalse())
				Expect(updated).To(BeTrue())
				Expect(warnings).To(ConsistOf([]concourse.ConfigWarning{
					{
						Type:    "warning-2-type",
						Message: "fake-warning2",
					},
					{
						Type:    "warning-1-type",
						Message: "fake-warning1",
					},
				}))
			})

			Context("when response contains bad JSON", func() {
				BeforeEach(func() {
					returnBody = []byte(`bad-json`)
				})

				It("returns an error", func() {
					_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when credential verification is enabled", func() {
				BeforeEach(func() {
					checkCredentials = true
				})

				Context("when credential doesn't exist in credentials manager", func() {
					BeforeEach(func() {
						returnHeader = http.StatusBadRequest
						returnBody = []byte(`{"errors":["Expected to find variables: BAR"]}`)
					})

					It("returns an error", func() {
						_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("Expected to find variables: BAR"))
					})
				})

			})
		})

		Context("when setting config returns bad request", func() {
			BeforeEach(func() {
				returnHeader = http.StatusBadRequest
				returnBody = []byte(`{"errors":["fake-error1","fake-error2"]}`)
			})

			It("returns config validation error", func() {
				_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid configuration:\n"))
				Expect(err.Error()).To(ContainSubstring("fake-error1\nfake-error2"))
			})

			Context("when response contains bad JSON", func() {
				BeforeEach(func() {
					returnBody = []byte(`bad-json`)
				})

				It("returns an error", func() {
					_, _, _, err := team.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedConfig, checkCredentials)
					Expect(err).To(HaveOccurred())
				})
			})
		})
	})
})
