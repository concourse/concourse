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

var _ = Describe("ATC Handler Builds", func() {
	Describe("CreateBuild", func() {
		var (
			plan          atc.Plan
			expectedBuild atc.Build
		)
		BeforeEach(func() {
			plan = atc.Plan{
				OnSuccess: &atc.OnSuccessPlan{
					Step: atc.Plan{
						Aggregate: &atc.AggregatePlan{},
					},
					Next: atc.Plan{
						ID: "some-guid",
						Task: &atc.TaskPlan{
							Name:       "one-off",
							Privileged: true,
							Config:     &atc.TaskConfig{},
						},
					},
				},
			}

			expectedBuild = atc.Build{
				ID:      123,
				Name:    "mybuild",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     "/builds/123",
				APIURL:  "/api/v1/builds/123",
			}
			expectedURL := "/api/v1/builds"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSONRepresenting(plan),
					ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedBuild),
				),
			)
		})

		It("takes a plan and creates the build", func() {
			build, err := client.CreateBuild(plan)
			Expect(err).NotTo(HaveOccurred())
			Expect(build).To(Equal(expectedBuild))
		})
	})

	Describe("CreateJobBuild", func() {
		var (
			pipelineName  string
			jobName       string
			expectedBuild atc.Build
		)
		BeforeEach(func() {
			pipelineName = "mypipeline"
			jobName = "myjob"

			expectedBuild = atc.Build{
				ID:      123,
				Name:    "mybuild",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild",
				APIURL:  "api/v1/builds/123",
			}
			expectedURL := "/api/v1/pipelines/mypipeline/jobs/myjob/builds"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedBuild),
				),
			)
		})

		It("takes a pipeline and a job and creates the build", func() {
			build, err := client.CreateJobBuild(pipelineName, jobName)
			Expect(err).NotTo(HaveOccurred())
			Expect(build).To(Equal(expectedBuild))
		})
	})

	Describe("JobBuild", func() {
		var (
			expectedBuild        atc.Build
			expectedURL          string
			expectedPipelineName string
		)

		Context("when build exists", func() {
			JustBeforeEach(func() {
				expectedBuild = atc.Build{
					ID:      123,
					Name:    "mybuild",
					Status:  "succeeded",
					JobName: "myjob",
					URL:     fmt.Sprint("/pipelines/", expectedPipelineName, "/jobs/myjob/builds/mybuild"),
					APIURL:  "api/v1/builds/123",
				}

				expectedURL = fmt.Sprint("/api/v1/pipelines/", expectedPipelineName, "/jobs/myjob/builds/mybuild")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
					),
				)
			})

			Context("when provided a pipline name", func() {
				BeforeEach(func() {
					expectedPipelineName = "mypipeline"
				})

				It("returns the given build", func() {
					build, found, err := client.JobBuild("mypipeline", "myjob", "mybuild")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(build).To(Equal(expectedBuild))
				})
			})

			Context("when not provided a pipeline name", func() {
				BeforeEach(func() {
					expectedPipelineName = "main"
				})

				It("errors", func() {
					_, _, err := client.JobBuild("", "myjob", "mybuild")
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when build does not exists", func() {
			BeforeEach(func() {
				expectedURL = "/api/v1/pipelines/mypipeline/jobs/myjob/builds/mybuild"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, nil),
					),
				)
			})

			It("return false and no error", func() {
				_, found, err := client.JobBuild("mypipeline", "myjob", "mybuild")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("Build", func() {
		Context("when build exists", func() {
			expectedBuild := atc.Build{
				ID:      123,
				Name:    "mybuild",
				Status:  "succeeded",
				JobName: "myjob",
				URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild",
				APIURL:  "api/v1/builds/123",
			}
			expectedURL := "/api/v1/builds/123"

			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
					),
				)
			})

			It("returns the given build", func() {
				build, found, err := client.Build("123")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build).To(Equal(expectedBuild))
			})
		})

		Context("when build does not exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/builds/123"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, nil),
					),
				)
			})

			It("returns false and no error", func() {
				_, found, err := client.Build("123")
				Expect(err).ToNot(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("Builds", func() {
		var (
			expectedBuilds []atc.Build
			expectedURL    string
			expectedQuery  string
		)

		BeforeEach(func() {
			expectedURL = ""
			expectedQuery = ""

			expectedBuilds = []atc.Build{
				{
					ID:      123,
					Name:    "mybuild1",
					Status:  "succeeded",
					JobName: "myjob",
					URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild1",
					APIURL:  "api/v1/builds/123",
				},
				{
					ID:      124,
					Name:    "mybuild2",
					Status:  "succeeded",
					JobName: "myjob",
					URL:     "/pipelines/mypipeline/jobs/myjob/builds/mybuild2",
					APIURL:  "api/v1/builds/124",
				},
			}
		})

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, expectedQuery),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
				),
			)
		})

		Context("when since, until, and limit are 0", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")
			})

			It("calls to get all builds", func() {
				builds, _, err := client.Builds(concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")
				expectedQuery = fmt.Sprint("since=24")
			})

			It("calls to get all builds since that id", func() {
				builds, _, err := client.Builds(concourse.Page{Since: 24})
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("since=24&limit=5")
				})

				It("appends limit to the url", func() {
					builds, _, err := client.Builds(concourse.Page{Since: 24, Limit: 5})
					Expect(err).NotTo(HaveOccurred())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when until is specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("calls to get all builds until that id", func() {
				builds, _, err := client.Builds(concourse.Page{Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("until=26&limit=15")
				})

				It("appends limit to the url", func() {
					builds, _, err := client.Builds(concourse.Page{Until: 26, Limit: 15})
					Expect(err).NotTo(HaveOccurred())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when since and until are both specified", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")
				expectedQuery = fmt.Sprint("until=26")
			})

			It("only sends the until", func() {
				builds, _, err := client.Builds(concourse.Page{Since: 24, Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false and an error", func() {
				_, _, err := client.Builds(concourse.Page{})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("pagination data", func() {
			Context("with a link header", func() {
				BeforeEach(func() {
					expectedURL = fmt.Sprint("/api/v1/builds")

					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL),
							ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{
								"Link": []string{
									`<http://some-url.com/api/v1/builds?since=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/builds?until=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					_, pagination, err := client.Builds(concourse.Page{})
					Expect(err).ToNot(HaveOccurred())
					Expect(pagination.Previous).To(Equal(&concourse.Page{Since: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{Until: 254, Limit: 456}))
				})
			})
		})

		Context("without a link header", func() {
			BeforeEach(func() {
				expectedURL = fmt.Sprint("/api/v1/builds")

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{}),
					),
				)
			})

			It("returns pagination data with nil pages", func() {
				_, pagination, err := client.Builds(concourse.Page{})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})

	Describe("AbortBuild", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/builds/123/abort"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWith(http.StatusNoContent, ""),
				),
			)
		})

		It("sends an abort request to ATC", func() {
			err := client.AbortBuild("123")
			Expect(err).NotTo(HaveOccurred())
			Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
		})
	})
})
