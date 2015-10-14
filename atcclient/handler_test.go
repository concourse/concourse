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

var _ = Describe("ATC Client Handler", func() {
	var (
		handler   atcclient.BuildHandler
		atcServer *ghttp.Server
		client    atcclient.Client
		config    atc.Config
	)

	BeforeEach(func() {
		var err error
		atcServer = ghttp.NewServer()
		config = atc.Config{}

		client, err = atcclient.NewClient(
			rc.NewTarget(atcServer.URL(), "", "", "", false),
		)
		Expect(err).NotTo(HaveOccurred())

		handler = atcclient.NewBuildHandler(client)
	})

	Describe("GetJobBuild", func() {
		expectedBuild := atc.Build{
			ID:      123,
			Name:    "mybuild",
			Status:  "succeeded",
			JobName: "myjob",
			URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild",
			ApiUrl:  "api/v1/builds/123",
		}
		expectedURL := "/pipelines/mypipeline/jobs/myjob/builds/mybuild"

		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedBuild, expectedURL),
					ghttp.RespondWithJSONEncoded(200, config, http.Header{atc.ConfigVersionHeader: {"42"}}),
				),
			)
		})

		FIt("returns the given build", func() {

			build, err := handler.GetJobBuild("myjob", "mybuild", "mypipeline")
			Expect(err).NotTo(HaveOccurred())

			// Expect(fakeClient.MakeRequestCallCount()).To(Equal(1))
			// buildInterface, requestName, params, queries := fakeClient.MakeRequestArgsForCall(0)
			// Expect(requestName).To(Equal("GetJobBuild"))
			// Expect(params).To(Equal(
			// 	map[string]string{
			// 		"job_name":      "myjob",
			// 		"build_name":    "mybuild",
			// 		"pipeline_name": "mypipeline",
			// 	},
			// ))
			// Expect(queries).To(BeNil())
			Expect(build).To(Equal(expectedBuild))
		})

		Context("with out a pipeline name", func() {
			It("uses the default pipeline name and returns the given build", func() {
				// handler.GetJobBuild("myjob", "mybuild", "")

				// Expect(fakeClient.MakeRequestCallCount()).To(Equal(1))
				// _, requestName, params, queries := fakeClient.MakeRequestArgsForCall(0)
				// Expect(requestName).To(Equal("GetJobBuild"))
				// Expect(params).To(Equal(
				// 	map[string]string{
				// 		"job_name":      "myjob",
				// 		"build_name":    "mybuild",
				// 		"pipeline_name": "main",
				// 	},
				// ))
				// Expect(queries).To(BeNil())
			})
		})
	})
})
