package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Configs", func() {
	Describe("PipelineConfig", func() {
		expectedURL := "/api/v1/pipelines/mypipeline/config"

		Context("ATC returns the correct response", func() {
			var (
				expectedConfig  atc.Config
				expectedVersion string
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

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedConfig, http.Header{atc.ConfigVersionHeader: {expectedVersion}}),
					),
				)
			})

			It("returns the given config and version for that pipeline", func() {
				pipelineConfig, version, err := handler.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelineConfig).To(Equal(expectedConfig))
				Expect(version).To(Equal(expectedVersion))
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
				_, _, err := handler.PipelineConfig("mypipeline")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("ATC does not return config version error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Config{}),
					),
				)
			})

			It("returns an empty value for the version", func() {
				_, version, err := handler.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(BeEmpty())
			})
		})
	})
})
