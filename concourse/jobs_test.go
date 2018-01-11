package concourse_test

import (
	"fmt"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Jobs", func() {
	Describe("team.ListJobs", func() {
		var expectedJobs []atc.Job

		Context("when pipeline name is empty", func() {
			BeforeEach(func() {
				expectedJobs = []atc.Job{}
			})

			It("returns empty job and name required error", func() {
				pipelines, err := team.ListJobs("")
				Expect(err).To(HaveOccurred())
				Expect(pipelines).To(Equal(expectedJobs))
			})
		})

		Context("when pipeline name is not empty", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/jobs"

				expectedJobs = []atc.Job{
					{
						Name:      "myjob-1",
						NextBuild: nil,
					},
					{
						Name:      "myjob-2",
						NextBuild: nil,
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedJobs),
					),
				)
			})

			It("returns jobs that belong to the pipeline", func() {
				pipelines, err := team.ListJobs("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelines).To(Equal(expectedJobs))
			})
		})
	})

	Describe("Job", func() {
		Context("when job exists", func() {
			var (
				expectedJob atc.Job
				expectedURL string
			)

			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob")

				expectedJob = atc.Job{
					Name:      "myjob",
					NextBuild: nil,
					FinishedBuild: &atc.Build{
						ID:      123,
						Name:    "mybuild",
						Status:  "succeeded",
						JobName: "myjob",
						APIURL:  "api/v1/teams/some-team/builds/123",
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

			It("returns the given job for that pipeline", func() {
				job, found, err := team.Job("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(job).To(Equal(expectedJob))
				Expect(found).To(BeTrue())
			})
		})

		Context("when job does not exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, found, err := team.Job("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("JobBuilds", func() {
		var (
			expectedBuilds []atc.Build
			expectedURL    string
			expectedQuery  string
		)

		JustBeforeEach(func() {
			expectedBuilds = []atc.Build{
				{
					Name: "some-build",
				},
				{
					Name: "some-other-build",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
				),
			)
		})

		Context("when since, until, and limit are 0", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")
			})

			It("calls to get all builds", func() {
				builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")
				expectedQuery = fmt.Sprint("since=24")
			})

			It("calls to get all builds since that id", func() {
				builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{Since: 24})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("since=24&limit=5")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{Since: 24, Limit: 5})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when until is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("calls to get all builds until that id", func() {
				builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("until=26&limit=15")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{Until: 26, Limit: 15})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when since and until are both specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("only sends the until", func() {
				builds, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{Since: 24, Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false and an error", func() {
				_, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{})
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns not found", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, _, found, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("pagination data", func() {
			Context("with a link header", func() {
				BeforeEach(func() {
					expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL),
							ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{
								"Link": []string{
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?since=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?until=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					_, pagination, _, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{})
					Expect(err).ToNot(HaveOccurred())

					Expect(pagination.Previous).To(Equal(&concourse.Page{Since: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{Until: 254, Limit: 456}))
				})
			})
		})

		Context("without a link header", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{}),
					),
				)
			})

			It("returns pagination data with nil pages", func() {
				_, pagination, _, err := team.JobBuilds("mypipeline", "myjob", concourse.Page{})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})

	Describe("PauseJob", func() {
		var (
			expectedStatus int
			pipelineName   = "banana"
			jobName        = "disjob"
			expectedURL    = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/jobs/%s/pause", pipelineName, jobName)
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the job exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the pause job and returns no error", func() {
				Expect(func() {
					paused, err := team.PauseJob(pipelineName, jobName)
					Expect(err).NotTo(HaveOccurred())
					Expect(paused).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the pause job call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the pause job and returns an error", func() {
				Expect(func() {
					paused, err := team.PauseJob(pipelineName, jobName)
					Expect(err).To(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the job does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the pause job and returns an error", func() {
				Expect(func() {
					paused, err := team.PauseJob(pipelineName, jobName)
					Expect(err).ToNot(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})

	Describe("UnpauseJob", func() {
		var (
			expectedStatus int
			pipelineName   = "banana"
			jobName        = "disjob"
			expectedURL    = fmt.Sprintf("/api/v1/teams/some-team/pipelines/%s/jobs/%s/unpause", pipelineName, jobName)
		)

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.RespondWith(expectedStatus, nil),
				),
			)
		})

		Context("when the job exists and there are no issues", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusOK
			})

			It("calls the pause job and returns no error", func() {
				Expect(func() {
					paused, err := team.UnpauseJob(pipelineName, jobName)
					Expect(err).NotTo(HaveOccurred())
					Expect(paused).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the pause job call fails", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusInternalServerError
			})

			It("calls the pause job and returns an error", func() {
				Expect(func() {
					paused, err := team.UnpauseJob(pipelineName, jobName)
					Expect(err).To(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the job does not exist", func() {
			BeforeEach(func() {
				expectedStatus = http.StatusNotFound
			})

			It("calls the pause job and returns an error", func() {
				Expect(func() {
					paused, err := team.UnpauseJob(pipelineName, jobName)
					Expect(err).ToNot(HaveOccurred())
					Expect(paused).To(BeFalse())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})
	})
})
