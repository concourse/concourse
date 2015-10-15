package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Config", func() {
	var (
		client    atcclient.Client
		handler   atcclient.AtcHandler
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error
		atcServer = ghttp.NewServer()

		client, err = atcclient.NewClient(
			rc.NewTarget(atcServer.URL(), "", "", "", false),
		)
		Expect(err).NotTo(HaveOccurred())

		handler = atcclient.NewAtcHandler(client)
	})

	AfterEach(func() {
		atcServer.Close()
	})

	Describe("Job", func() {

		var (
			expectedConfig atc.Config
			expectedURL    string
		)

		JustBeforeEach(func() {
			expectedURL = "/api/v1/pipelines/mypipeline/config"

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
						Name: "some-job",

						Public: true,

						TaskConfigPath: "some/config/path.yml",
						TaskConfig: &atc.TaskConfig{
							Image: "some-image",
							Params: map[string]string{
								"A": "B",
							},
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
					{
						Name: "some-other-job",

						TaskConfigPath: "some/config/path.yml",

						InputConfigs: []atc.JobInputConfig{
							{
								RawName:  "some-other-input",
								Resource: "some-other-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								Passed: []string{"job-1", "job-2"},
							},
						},

						OutputConfigs: []atc.JobOutputConfig{
							{
								Resource: "some-other-resource",
								Params: atc.Params{
									"some-param": "some-value",
								},
								RawPerformOn: []atc.Condition{"success", "failure"},
							},
						},
					},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, expectedConfig, http.Header{}),
				),
			)
		})

		It("returns the given config for that pipeline", func() {
			pipelineConfig, err := handler.PipelineConfig("mypipeline")
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelineConfig).To(Equal(expectedConfig))
		})
	})
})
