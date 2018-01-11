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
				APIURL:  "api/v1/builds/123",
			}
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedBuild),
				),
			)
		})

		It("takes a pipeline and a job and creates the build", func() {
			build, err := team.CreateJobBuild(pipelineName, jobName)
			Expect(err).NotTo(HaveOccurred())
			Expect(build).To(Equal(expectedBuild))
		})
	})

	Describe("JobBuild", func() {
		var (
			expectedBuild atc.Build
			expectedURL   string
		)

		Context("when build exists", func() {
			BeforeEach(func() {
				expectedBuild = atc.Build{
					ID:      123,
					Name:    "mybuild",
					Status:  "succeeded",
					JobName: "myjob",
					APIURL:  "api/v1/builds/123",
				}

				expectedURL = "/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds/mybuild"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuild),
					),
				)
			})

			It("returns the given build", func() {
				build, found, err := team.JobBuild("mypipeline", "myjob", "mybuild")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build).To(Equal(expectedBuild))
			})
		})

		Context("when build does not exist", func() {
			BeforeEach(func() {
				expectedURL = "/api/v1/teams/some-team/pipelines/mypipeline/jobs/myjob/builds/mybuild"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, nil),
					),
				)
			})

			It("return false and no error", func() {
				_, found, err := team.JobBuild("mypipeline", "myjob", "mybuild")
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
		expectedURL := "/api/v1/builds"

		var expectedBuilds []atc.Build

		var page concourse.Page

		var builds []atc.Build
		var pagination concourse.Pagination
		var clientErr error

		BeforeEach(func() {
			page = concourse.Page{}

			expectedBuilds = []atc.Build{
				{
					ID:      123,
					Name:    "mybuild1",
					Status:  "succeeded",
					JobName: "myjob",
					APIURL:  "api/v1/builds/123",
				},
				{
					ID:      124,
					Name:    "mybuild2",
					Status:  "succeeded",
					JobName: "myjob",
					APIURL:  "api/v1/builds/124",
				},
			}
		})

		JustBeforeEach(func() {
			builds, pagination, clientErr = client.Builds(page)
		})

		Context("when since, until, and limit are 0", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("calls to get all builds", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{Since: 24}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "since=24"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("calls to get all builds since that id", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since and limit is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{Since: 24, Limit: 5}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "since=24&limit=5"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("appends limit to the url", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when until is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{Until: 26}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "until=26"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("calls to get all builds until that id", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when until and limit is specified", func() {
			BeforeEach(func() {
				page = concourse.Page{Until: 26, Limit: 15}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "until=26&limit=15"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("appends limit to the url", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since and until are both specified", func() {
			BeforeEach(func() {
				page = concourse.Page{Since: 24, Until: 26}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, "until=26"),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
					),
				)
			})

			It("only sends the until", func() {
				Expect(clientErr).NotTo(HaveOccurred())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when the server returns an error", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusInternalServerError, ""),
					),
				)
			})

			It("returns false and an error", func() {
				Expect(clientErr).To(HaveOccurred())
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
					Expect(clientErr).ToNot(HaveOccurred())
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
				Expect(clientErr).ToNot(HaveOccurred())
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
					ghttp.VerifyRequest("PUT", expectedURL),
					ghttp.RespondWith(http.StatusNoContent, ""),
				),
			)
		})

		It("sends an abort request to ATC", func() {
			Expect(func() {
				err := client.AbortBuild("123")
				Expect(err).NotTo(HaveOccurred())
			}).To(Change(func() int {
				return len(atcServer.ReceivedRequests())
			}).By(1))
		})
	})
})
