package atcclient_test

import (
	"bytes"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/atcclient/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Configs", func() {
	Describe("PipelineConfig", func() {
		expectedURL := "/api/v1/pipelines/mypipeline/config"

		Context("ATC returns the correct response when it exists", func() {
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
				pipelineConfig, version, found, err := handler.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelineConfig).To(Equal(expectedConfig))
				Expect(version).To(Equal(expectedVersion))
				Expect(found).To(BeTrue())
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
				_, _, found, err := handler.PipelineConfig("mypipeline")
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
				_, _, _, err := handler.PipelineConfig("mypipeline")
				Expect(err).To(HaveOccurred())
			})
		})

		Context("ATC does not return config version", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, atc.Config{}),
					),
				)
			})

			It("returns an error", func() {
				_, _, _, err := handler.PipelineConfig("mypipeline")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("CreateOrUpdatePipelineConfig", func() {
		Context("when creating a new config", func() {
			var (
				fakeClient           *fakes.FakeClient
				expectedPipelineName string
				expectedBody         *bytes.Buffer
				expectedVersion      string
				expectedContentType  string
			)
			BeforeEach(func() {
				fakeClient = new(fakes.FakeClient)
				handler = atcclient.NewAtcHandler(fakeClient)

				expectedPipelineName = "mypipeline"
				expectedBody = &bytes.Buffer{}
				expectedVersion = "42"
				expectedContentType = "applicatione/fakefakefake"

				expectedRequest := atcclient.Request{
					RequestName: atc.SaveConfig,
					Params: map[string]string{
						"pipeline_name": expectedPipelineName,
					},
					Body: expectedBody,
					Headers: map[string][]string{
						"Content-Type":          {expectedContentType},
						atc.ConfigVersionHeader: {expectedVersion},
					},
				}

				fakeClient.SendStub = func(request atcclient.Request, response *atcclient.Response) error {
					Expect(request).To(Equal(expectedRequest))
					response.Created = true
					return nil
				}
			})

			It("returns true for created and false for updated", func() {
				created, updated, err := handler.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedBody, expectedContentType)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeTrue())
				Expect(updated).To(BeFalse())
			})
		})

		Context("when updating a config", func() {
			var (
				fakeClient           *fakes.FakeClient
				expectedPipelineName string
				expectedBody         *bytes.Buffer
				expectedVersion      string
				expectedContentType  string
			)
			BeforeEach(func() {
				fakeClient = new(fakes.FakeClient)
				handler = atcclient.NewAtcHandler(fakeClient)

				expectedPipelineName = "mypipeline"
				expectedBody = &bytes.Buffer{}
				expectedVersion = "42"
				expectedContentType = "applicatione/fakefakefake"

				expectedRequest := atcclient.Request{
					RequestName: atc.SaveConfig,
					Params: map[string]string{
						"pipeline_name": expectedPipelineName,
					},
					Body: expectedBody,
					Headers: map[string][]string{
						"Content-Type":          {expectedContentType},
						atc.ConfigVersionHeader: {expectedVersion},
					},
				}

				fakeClient.SendStub = func(request atcclient.Request, response *atcclient.Response) error {
					Expect(request).To(Equal(expectedRequest))
					return nil
				}
			})

			It("returns false for created and true for updated", func() {
				created, updated, err := handler.CreateOrUpdatePipelineConfig(expectedPipelineName, expectedVersion, expectedBody, expectedContentType)
				Expect(err).NotTo(HaveOccurred())
				Expect(created).To(BeFalse())
				Expect(updated).To(BeTrue())
			})
		})
	})
})
