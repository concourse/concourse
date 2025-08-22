package concourse_test

import (
	"net/http"
	"strings"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipelines", func() {
	Describe("PausePipeline", func() {

		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/pause"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.PausePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.PausePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("ArchivePipeline", func() {

		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/archive"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.ArchivePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				found, err := team.ArchivePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("UnpausePipeline", func() {

		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/unpause"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.UnpausePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.UnpausePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("ExposePipeline", func() {

		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/expose"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.ExposePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.ExposePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("HidePipeline", func() {

		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/hide"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
					),
				)
			})

			It("return true and no error", func() {
				found, err := team.HidePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
			})
		})

		Context("when the pipeline doesn't exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns false and no error", func() {
				found, err := team.HidePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("OrderingPipelines", func() {
		Context("when the API call succeeds", func() {
			orderedPipelines := []string{
				"mypipeline1",
				"mypipeline2",
			}

			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/ordering"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(orderedPipelines),
						ghttp.RespondWith(http.StatusOK, ""),
					),
				)
			})

			It("return no error", func() {
				err := team.OrderingPipelines(orderedPipelines)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the API call errors", func() {
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
				err := team.OrderingPipelines([]string{"p"})
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("OrderingInstancePipelines", func() {
		instanceVars := []atc.InstanceVars{
			{"branch": "main"},
			{"branch": "test"},
		}

		Context("when the API call succeeds", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/my-pipeline/ordering"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSONRepresenting(instanceVars),
						ghttp.RespondWith(http.StatusOK, ""),
					),
				)
			})

			It("return no error", func() {
				err := team.OrderingPipelinesWithinGroup("my-pipeline", instanceVars)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the API call errors", func() {
			BeforeEach(func() {
				expectedURL := "/api/v1/teams/some-team/pipelines/my-pipeline/ordering"
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
					),
				)
			})
			It("returns error", func() {
				err := team.OrderingPipelinesWithinGroup("my-pipeline", instanceVars)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Pipeline", func() {
		var expectedPipeline atc.Pipeline
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

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
						ghttp.VerifyRequest("GET", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedPipeline),
					),
				)
			})

			It("returns the requested pipeline", func() {
				pipeline, found, err := team.Pipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(pipeline).To(Equal(expectedPipeline))
			})
		})

		Context("when the pipeline is not found", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL, queryParams),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false", func() {
				_, found, err := team.Pipeline(pipelineRef)
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
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL, queryParams),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("deletes the pipeline when called", func() {
				Expect(func() {
					found, err := team.DeletePipeline(pipelineRef)
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
						ghttp.VerifyRequest("DELETE", expectedURL, queryParams),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				found, err := team.DeletePipeline(pipelineRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("RenamePipeline", func() {
		var (
			expectedURL         string
			expectedRequestBody string
			expectedResponse    atc.SaveConfigResponse
			pipelineName        string
		)

		BeforeEach(func() {
			expectedURL = "/api/v1/teams/some-team/pipelines/mypipeline/rename"
			expectedRequestBody = `{"name":"newpipelinename"}`
			expectedResponse = atc.SaveConfigResponse{
				Errors:       nil,
				ConfigErrors: []atc.ConfigErrors{},
			}
			pipelineName = "mypipeline"
		})

		Context("when the pipeline exists", func() {
			JustBeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", expectedURL),
						ghttp.VerifyJSON(expectedRequestBody),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedResponse),
					),
				)
			})

			It("renames the pipeline when called", func() {
				renamed, _, err := team.RenamePipeline(pipelineName, "newpipelinename")
				Expect(err).NotTo(HaveOccurred())
				Expect(renamed).To(BeTrue())
			})

			Context("when the pipeline identifier is invalid", func() {
				BeforeEach(func() {
					expectedRequestBody = `{"name":"_newpipelinename"}`
					expectedResponse = atc.SaveConfigResponse{
						Errors: nil,
						ConfigErrors: []atc.ConfigErrors{
							{
								Type:    "invalid_identifier",
								Message: "pipeline: '_newpipelinename' is not a valid identifier",
							},
						},
					}
				})

				It("returns a warning", func() {
					renamed, warnings, err := team.RenamePipeline(pipelineName, "_newpipelinename")
					Expect(err).ToNot(HaveOccurred())
					Expect(renamed).To(BeTrue())
					Expect(warnings).To(HaveLen(0))
				})
			})
		})

		Context("when the pipeline does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.RespondWith(http.StatusNotFound, ""),
				)
			})

			It("returns false and no error", func() {
				renamed, _, err := team.RenamePipeline(pipelineName, "newpipelinename")
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
				renamed, _, err := team.RenamePipeline(pipelineName, "newpipelinename")
				Expect(err).To(MatchError(ContainSubstring("418 I'm a teapot")))
				Expect(renamed).To(BeFalse())
			})
		})
	})

	Describe("CreatePipelineBuild", func() {
		expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/builds"
		queryParams := "vars.branch=%22master%22"
		pipelineRef := atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}

		var (
			plan          atc.Plan
			expectedBuild atc.Build
		)
		Context("When the build is created", func() {
			BeforeEach(func() {
				plan = atc.Plan{
					OnSuccess: &atc.OnSuccessPlan{
						Step: atc.Plan{
							InParallel: &atc.InParallelPlan{},
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
						ghttp.VerifyRequest("POST", expectedURL, queryParams),
						ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedBuild),
					),
				)
			})

			It("returns the build and no error", func() {
				build, err := team.CreatePipelineBuild(pipelineRef, plan)
				Expect(err).NotTo(HaveOccurred())
				Expect(build).To(Equal(expectedBuild))
			})
		})
	})

	Describe("PipelineBuilds", func() {
		var (
			expectedBuilds []atc.Build
			expectedURL    string
			expectedQuery  []string
			pipelineRef    atc.PipelineRef
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

			expectedURL = "/api/v1/teams/some-team/pipelines/mypipeline/builds"
			expectedQuery = []string{"vars.branch=%22master%22"}
			pipelineRef = atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
		})

		JustBeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL, strings.Join(expectedQuery, "&")),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuilds),
				),
			)
		})

		Context("when from, to, and limit are 0", func() {
			BeforeEach(func() {
			})

			It("calls to get all builds", func() {
				builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})
		})

		Context("when from is specified", func() {
			BeforeEach(func() {
				expectedQuery = append(expectedQuery, "from=24")
			})

			It("calls to get all builds from that id", func() {
				builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{From: 24})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = append(expectedQuery, "limit=5")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{From: 24, Limit: 5})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when to is specified", func() {
			BeforeEach(func() {
				expectedQuery = append(expectedQuery, "to=26")
			})

			It("calls to get all builds to that id", func() {
				builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{To: 26})
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(builds).To(Equal(expectedBuilds))
			})

			Context("and limit is specified", func() {
				BeforeEach(func() {
					expectedQuery = append(expectedQuery, "limit=15")
				})

				It("appends limit to the url", func() {
					builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{To: 26, Limit: 15})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(builds).To(Equal(expectedBuilds))
				})
			})
		})

		Context("when from and to are both specified", func() {
			BeforeEach(func() {
				expectedQuery = append(expectedQuery, "to=26", "from=24")
			})

			It("sends both from and to", func() {
				builds, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{From: 24, To: 26})
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
				_, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{})
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
				_, _, found, err := team.PipelineBuilds(pipelineRef, concourse.Page{})
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
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/builds?from=452&limit=123>; rel="previous"`,
									`<http://some-url.com/api/v1/teams/some-team/pipelines/some-pipeline/builds?to=254&limit=456>; rel="next"`,
								},
							}),
						),
					)
				})

				It("returns the pagination data from the header", func() {
					_, pagination, _, err := team.PipelineBuilds(pipelineRef, concourse.Page{})
					Expect(err).ToNot(HaveOccurred())

					Expect(pagination.Previous).To(Equal(&concourse.Page{From: 452, Limit: 123}))
					Expect(pagination.Next).To(Equal(&concourse.Page{To: 254, Limit: 456}))
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
				_, pagination, _, err := team.PipelineBuilds(pipelineRef, concourse.Page{})
				Expect(err).ToNot(HaveOccurred())

				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(BeNil())
			})
		})
	})
})
