package atcclient_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Jobs", func() {
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
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedJob),
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
