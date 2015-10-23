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
		Context("when job exists", func() {
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
					job, found, err := handler.Job("mypipeline", "myjob")
					Expect(err).NotTo(HaveOccurred())
					Expect(job).To(Equal(expectedJob))
					Expect(found).To(BeTrue())
				})
			})
		})

		Context("when job does not exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/pipelines/mypipeline/jobs/myjob"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, found, err := handler.Job("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
