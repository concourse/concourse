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

var _ = Describe("ATC Handler Pipelines", func() {
	Describe("PausePipeline", func() {
		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/pause"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.PausePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/pause"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.PausePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("UnpausePipeline", func() {
		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/unpause"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.UnpausePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/unpause"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.UnpausePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("ExposePipeline", func() {
		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/expose"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.ExposePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/expose"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.ExposePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("HidePipeline", func() {
		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/hide"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.HidePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/hide"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.HidePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("OrderingPipelines", func() {
		Context("when the pipelines exists", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/ordering"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return no error", func() {
				err := team.OrderingPipelines([]string{"mypipeline", "mypipeline2"})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/ordering"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, ""),
					),
				)
			})
			It("returns error", func() {
				err := team.OrderingPipelines([]string{"mypipeline"})
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the team doesn't exist", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/ordering"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns error", func() {
				err := team.OrderingPipelines([]string{"mypipeline"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Pipeline", func() {
		var expectedPipeline atc.Pipeline
		pipelineName := "mypipeline"
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline"

		BeforeEach(func() {
			expectedPipeline = atc.Pipeline{
				Name:   "mypipeline",
				Paused: true,
				Groups: []atc.GroupConfig{
					{
						Name:      "group1",
						Jobs:      []string{"job1", "job2"},
						Resources: []string{"resource1", "resource2"},
					},
				},
			}
		})

		Context("when the pipeline is found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedPipeline),
					),
				)
			})

			It("returns the requested pipeline", func() {
				pipeline, found, err := team.Pipeline(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline).To(Equal(expectedPipeline))
			})
		})

		Context("when the pipeline is not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false", func() {
				_, found, err := team.Pipeline(pipelineName)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("team.ListPipelines", func() {
		var expectedPipelines []atc.Pipeline

		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines"

			expectedPipelines = []atc.Pipeline{
				{
					Name:   "mypipeline-1",
					Paused: true,
					Groups: []atc.GroupConfig{
						{
							Name:      "group1",
							Jobs:      []string{"job1", "job2"},
							Resources: []string{"resource1", "resource2"},
						},
					},
				},
				{
					Name:   "mypipeline-2",
					Paused: false,
					Groups: []atc.GroupConfig{
						{
							Name:      "group2",
							Jobs:      []string{"job3", "job4"},
							Resources: []string{"resource3", "resource4"},
						},
					},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedPipelines),
				),
			)
		})

		It("returns pipelines that belong to team", func() {
			pipelines, err := team.ListPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedPipelines))
		})
	})

	Describe("client.ListPipelines", func() {
		var expectedPipelines []atc.Pipeline

		BeforeEach(func() {
			expectedURL := "/api/v1/pipelines"

			expectedPipelines = []atc.Pipeline{
				{
					Name:   "mypipeline-1",
					Paused: true,
					Groups: []atc.GroupConfig{
						{
							Name:      "group1",
							Jobs:      []string{"job1", "job2"},
							Resources: []string{"resource1", "resource2"},
						},
					},
				},
				{
					Name:   "mypipeline-2",
					Paused: false,
					Groups: []atc.GroupConfig{
						{
							Name:      "group2",
							Jobs:      []string{"job3", "job4"},
							Resources: []string{"resource3", "resource4"},
						},
					},
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedPipelines),
				),
			)
		})

		It("returns all the pipelines", func() {
			pipelines, err := client.ListPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedPipelines))
		})
	})

	Describe("DeletePipeline", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline"

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("deletes the pipeline when called", func() {
				Expect(func() {
					found, err := team.DeletePipeline("mypipeline")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
				}).To(Change(func() int {
					return len(atcServer.ReceivedRequests())
				}).By(1))
			})
		})

		Context("when the pipeline does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				found, err := team.DeletePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("RenamePipeline", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/rename"

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSON(`{"name":"newpipelinename"}`),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("renames the pipeline when called", func() {
				renamed, err := team.RenamePipeline("mypipeline", "newpipelinename")
				Expect(err).NotTo(HaveOccurred())
				Expect(renamed).To(BeTrue())
			})
		})

		Context("when the pipeline does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.RespondWith(http.StatusNotFound, ""),
				)
			})

			It("returns false and no error", func() {
				renamed, err := team.RenamePipeline("mypipeline", "newpipelinename")
				Expect(err).NotTo(HaveOccurred())
				Expect(renamed).To(BeFalse())
			})
		})

		Context("when an error occurs", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.RespondWith(http.StatusTeapot, ""),
				)
			})

			It("returns an error", func() {
				renamed, err := team.RenamePipeline("mypipeline", "newpipelinename")
				Expect(err).To(MatchError(ContainSubstring("418 I'm a teapot")))
				Expect(renamed).To(BeFalse())
			})
		})
	})

	Describe("CreatePipelineBuild", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/builds"

		var (
			plan          atc.Plan
			expectedBuild atc.Build
		)
		Context("When the build is created", func() {
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
					ID:           123,
					Name:         "mybuild",
					Status:       "succeeded",
					PipelineName: "mypipeline",
					APIURL:       "/api/v1/builds/123",
				}
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedBuild),
					),
				)
			})

			It("returns the build and no error", func() {
				build, err := team.CreatePipelineBuild("mypipeline", plan)
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedBuild))
			})
		})
	})

	Describe("PipelineBuilds", func() {
		var (
			expectedBuilds []atc.Build
			expectedURL    string
			expectedQuery  string
		)

		BeforeEach(func() {
			expectedBuilds = []atc.Build{
				{
					Name: "some-build",
				},
				{
					Name: "some-other-build",
				},
			}

			expectedURL = fmt.Sprint("/api/v1/teams/some-team/pipelines/mypipeline/builds")
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
			})

			It("calls to get all builds", func() {
				builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when since is specified", func() {
			BeforeEach(func() {
				expectedQuery = fmt.Sprint("since=24")
			})

			It("calls to get all builds since that id", func() {
				builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{Since: 24})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("since=24&limit=5")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{Since: 24, Limit: 5})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when until is specified", func() {
			BeforeEach(func() {
				expectedQuery = fmt.Sprint("until=26")
			})

			It("calls to get all builds until that id", func() {
				builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = fmt.Sprint("until=26&limit=15")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{Until: 26, Limit: 15})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when since and until are both specified", func() {
			BeforeEach(func() {
				expectedQuery = fmt.Sprint("until=26")
			})

			It("only sends the until", func() {
				builds, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{Since: 24, Until: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
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
				_, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{})
				Expect(err).To(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when the server returns not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				_, _, found, err := team.PipelineBuilds("mypipeline", concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("pagination data", func() {
			Context("with a link header", func() {
				BeforeEach(func() {
					atcServer.AppendHandlers(
						ghttp.CombineHandlers(
							ghttp.VerifyRequest("GET", expectedURL),
							ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{
								"Link": []string{
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/builds?since=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/builds?until=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					_, pagination, _, err := team.PipelineBuilds("mypipeline", concourse.Page{})
					Expect(err).ToNot(HaveOccurred())

					Expect(pagination.Previous).To(Equal(&concourse.Page{Since: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{Until: 254, Limit: 456}))
				})
			})
		})

		Context("without a link header", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds, http.Header{}),
					),
				)
			})

			It("returns pagination data with nil pages", func() {
				_, pagination, _, err := team.PipelineBuilds("mypipeline", concourse.Page{})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})
})
