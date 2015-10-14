package atcclient_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATCClient ATC Handler", func() {
	var (
		handler   atcclient.AtcHandler
		atcServer *ghttp.Server
		client    atcclient.Client
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

	Describe("JobBuild", func() {
		var (
			expectedBuild        atc.Build
			expectedURL          string
			expectedPipelineName string
		)

		JustBeforeEach(func() {
			expectedBuild = atc.Build{
				ID:      123,
				Name:    "mybuild",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     fmt.Sprint("/pipelines/", expectedPipelineName, "/jobs/myjob/builds/mybuild"),
				ApiUrl:  "api/v1/builds/123",
			}

			expectedURL = fmt.Sprint("/api/v1/pipelines/", expectedPipelineName, "/jobs/myjob/builds/mybuild")

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, expectedBuild, http.Header{}),
				),
			)
		})

		Context("when provided a pipline name", func() {
			BeforeEach(func() {
				expectedPipelineName = "mypipeline"
			})

			It("returns the given build", func() {
				build, err := handler.JobBuild("mypipeline", "myjob", "mybuild")
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedBuild))
			})
		})

		Context("when not provided a pipeline name", func() {
			BeforeEach(func() {
				expectedPipelineName = "main"
			})

			It("returns the given build for the default pipeline 'main'", func() {
				build, err := handler.JobBuild("", "myjob", "mybuild")
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedBuild))
			})
		})
	})

	Describe("Build", func() {
		expectedBuild := atc.Build{
			ID:      123,
			Name:    "mybuild",
			Status:  "succeeded",
			JobName: "myjob",
			URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild",
			ApiUrl:  "api/v1/builds/123",
		}
		expectedURL := "/api/v1/builds/123"

		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, expectedBuild, http.Header{}),
				),
			)
		})

		It("returns the given build", func() {
			build, err := handler.Build("123")
			Expect(err).NotTo(HaveOccurred())
			Expect(build).To(Equal(expectedBuild))
		})
	})

	Describe("AllBuilds", func() {
		expectedBuilds := []atc.Build{
			{
				ID:      123,
				Name:    "mybuild1",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild1",
				ApiUrl:  "api/v1/builds/123",
			},
			{
				ID:      124,
				Name:    "mybuild2",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild2",
				ApiUrl:  "api/v1/builds/124",
			},
		}
		expectedURL := "/api/v1/builds"

		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, expectedBuilds, http.Header{}),
				),
			)
		})

		It("returns the all the builds", func() {
			build, err := handler.AllBuilds()
			Expect(err).NotTo(HaveOccurred())
			Expect(build).To(Equal(expectedBuilds))
		})
	})

	Describe("Job", func() {

		var (
			expectedPipelineName string
			expectedJob          atc.Job
			expectedURL          string
		)

		JustBeforeEach(func() {
			expectedURL = fmt.Sprint("/api/v1/pipelines/", expectedPipelineName, "/jobs/myjob")

			expectedJob = atc.Job{
				Name:      "myjob",
				URL:       fmt.Sprint("/pipelines/", expectedPipelineName, "/jobs/myjob"),
				NextBuild: nil,
				FinishedBuild: &atc.Build{
					ID:      123,
					Name:    "mybuild",
					Status:  "succeeded",
					JobName: "myjob",
					URL:     fmt.Sprint("/pipelines/", expectedPipelineName, "/jobs/myjob/builds/mybuild"),
					ApiUrl:  "api/v1/builds/123",
				},
				Inputs: []atc.JobInput{
					{
						Name:     "myfirstinput",
						Resource: "myfirstinput",
						Passed:   []string{"rc"},
						Trigger:  true,
					},
					{
						Name:     "mysecondinput",
						Resource: "mysecondinput",
						Passed:   []string{"rc"},
						Trigger:  true,
					},
				},
				Outputs: []atc.JobOutput{
					{
						Name:     "myfirstoutput",
						Resource: "myfirstoutput",
					},
					{
						Name:     "mysecoundoutput",
						Resource: "mysecoundoutput",
					},
				},
				Groups: []string{"mygroup"},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(200, expectedJob, http.Header{}),
				),
			)
		})

		Context("when provided a pipline name", func() {
			BeforeEach(func() {
				expectedPipelineName = "mypipeline"
			})

			It("returns the given job for that pipeline", func() {
				job, err := handler.Job("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(job).To(Equal(expectedJob))
			})
		})

		Context("when not provided a pipeline name", func() {
			BeforeEach(func() {
				expectedPipelineName = "main"
			})

			It("returns the given job for the default pipeline 'main'", func() {
				job, err := handler.Job("", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(job).To(Equal(expectedJob))
			})
		})
	})
})
