package api_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {
	var (
		dbPipeline *dbfakes.FakePipeline
		fakeTeam   *dbfakes.FakeTeam

		publicPipeline                 *dbfakes.FakePipeline
		anotherPublicPipeline          *dbfakes.FakePipeline
		privatePipeline                *dbfakes.FakePipeline
		privatePipelineFromAnotherTeam *dbfakes.FakePipeline
	)
	BeforeEach(func() {
		dbPipeline = new(dbfakes.FakePipeline)
		fakeTeam = new(dbfakes.FakeTeam)
		publicPipeline = new(dbfakes.FakePipeline)

		publicPipeline.IDReturns(1)
		publicPipeline.PausedReturns(true)
		publicPipeline.PublicReturns(true)
		publicPipeline.TeamNameReturns("main")
		publicPipeline.NameReturns("public-pipeline")
		publicPipeline.GroupsReturns(atc.GroupConfigs{
			{
				Name:      "group2",
				Jobs:      []string{"job3", "job4"},
				Resources: []string{"resource3", "resource4"},
			},
		})
		publicPipeline.DisplayReturns(&atc.DisplayConfig{
			BackgroundImage: "background.jpg",
		})
		publicPipeline.LastUpdatedReturns(time.Unix(1, 0))

		anotherPublicPipeline = new(dbfakes.FakePipeline)
		anotherPublicPipeline.IDReturns(2)
		anotherPublicPipeline.PausedReturns(true)
		anotherPublicPipeline.PublicReturns(true)
		anotherPublicPipeline.TeamNameReturns("another")
		anotherPublicPipeline.NameReturns("another-pipeline")
		anotherPublicPipeline.LastUpdatedReturns(time.Unix(1, 0))

		privatePipeline = new(dbfakes.FakePipeline)
		privatePipeline.IDReturns(3)
		privatePipeline.PausedReturns(false)
		privatePipeline.PublicReturns(false)
		privatePipeline.ArchivedReturns(true)
		privatePipeline.TeamNameReturns("main")
		privatePipeline.NameReturns("private-pipeline")
		privatePipeline.GroupsReturns(atc.GroupConfigs{
			{
				Name:      "group1",
				Jobs:      []string{"job1", "job2"},
				Resources: []string{"resource1", "resource2"},
			},
		})
		privatePipeline.LastUpdatedReturns(time.Unix(1, 0))

		privatePipelineFromAnotherTeam = new(dbfakes.FakePipeline)
		privatePipelineFromAnotherTeam.IDReturns(3)
		privatePipelineFromAnotherTeam.PausedReturns(false)
		privatePipelineFromAnotherTeam.PublicReturns(false)
		privatePipelineFromAnotherTeam.TeamNameReturns("main")
		privatePipelineFromAnotherTeam.NameReturns("private-pipeline")
		privatePipelineFromAnotherTeam.LastUpdatedReturns(time.Unix(1, 0))

		fakeTeam.PipelinesReturns([]db.Pipeline{
			privatePipeline,
			publicPipeline,
		}, nil)

		fakeTeam.PublicPipelinesReturns([]db.Pipeline{publicPipeline}, nil)

		dbPipelineFactory.VisiblePipelinesReturns([]db.Pipeline{publicPipeline, anotherPublicPipeline}, nil)
	})

	Describe("GET /api/v1/pipelines", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/pipelines", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			expectedHeaderEntries := map[string]string{
				"Content-Type": "application/json",
			}
			Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
		})

		It("returns a JSON array of pipeline objects", func() {
			body, err := io.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(body).To(MatchJSON(`[
				{
					"id": 1,
					"name": "public-pipeline",
					"paused": true,
					"public": true,
					"archived": false,
					"team_name": "main",
					"last_updated": 1,
					"groups": [
						{
							"name": "group2",
							"jobs": ["job3", "job4"],
							"resources": ["resource3", "resource4"]
						}
					],
					"display": {
						"background_image": "background.jpg"
					}
				},
				{
					"id": 2,
					"name": "another-pipeline",
					"paused": true,
					"public": true,
					"archived": false,
					"team_name": "another",
					"last_updated": 1
				}
			]`))
		})

		Context("when team is set in user context", func() {
			BeforeEach(func() {
				fakeAccess.TeamNamesReturns([]string{"some-team"})
			})

			It("constructs pipeline factory with provided team names", func() {
				Expect(dbPipelineFactory.VisiblePipelinesCallCount()).To(Equal(1))
				Expect(dbPipelineFactory.VisiblePipelinesArgsForCall(0)).To(ContainElement("some-team"))
			})
		})

		Context("when not authenticated", func() {
			It("returns only public pipelines", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				var pipelines []map[string]any
				err = json.Unmarshal(body, &pipelines)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelines).To(ConsistOf(
					HaveKeyWithValue("id", BeNumerically("==", publicPipeline.ID())),
					HaveKeyWithValue("id", BeNumerically("==", anotherPublicPipeline.ID())),
				))
			})

			It("populates pipeline factory with no team names", func() {
				Expect(dbPipelineFactory.VisiblePipelinesCallCount()).To(Equal(1))
				Expect(dbPipelineFactory.VisiblePipelinesArgsForCall(0)).To(BeEmpty())
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.TeamNamesReturns([]string{"main"})
				dbPipelineFactory.VisiblePipelinesReturns([]db.Pipeline{privatePipeline, publicPipeline, anotherPublicPipeline}, nil)
			})

			It("returns all pipelines of the team + all public pipelines", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(dbPipelineFactory.VisiblePipelinesCallCount()).To(Equal(1))

				var pipelines []map[string]any
				err = json.Unmarshal(body, &pipelines)
				Expect(err).NotTo(HaveOccurred())
				Expect(pipelines).To(ConsistOf(
					HaveKeyWithValue("id", BeNumerically("==", publicPipeline.ID())),
					HaveKeyWithValue("id", BeNumerically("==", privatePipeline.ID())),
					HaveKeyWithValue("id", BeNumerically("==", anotherPublicPipeline.ID())),
				))
			})

			Context("user has the Admin privilege", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
					dbPipelineFactory.AllPipelinesReturns(
						[]db.Pipeline{publicPipeline, privatePipeline, anotherPublicPipeline, privatePipelineFromAnotherTeam},
						nil)
				})

				It("user can see all private and public pipelines from all teams", func() {
					Expect(dbPipelineFactory.AllPipelinesCallCount()).To(Equal(1),
						"Expected AllPipelines() to be called once")

					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					var pipelinesResponse []atc.Pipeline
					err = json.Unmarshal(body, &pipelinesResponse)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(pipelinesResponse)).To(Equal(4))
				})
			})

			Context("when the call to get active pipelines fails", func() {
				BeforeEach(func() {
					dbPipelineFactory.VisiblePipelinesReturns(nil, errors.New("disaster"))
				})

				It("returns 500 internal server error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/main/pipelines", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated as requested team", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			It("constructs team with provided team name", func() {
				Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
				Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("main"))
			})

			It("returns a JSON array of pipeline objects", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				Expect(body).To(MatchJSON(`[
					{
						"id": 3,
						"name": "private-pipeline",
						"paused": false,
						"public": false,
						"archived": true,
						"team_name": "main",
						"last_updated": 1,
						"groups": [
							{
								"name": "group1",
								"jobs": ["job1", "job2"],
								"resources": ["resource1", "resource2"]
							}
						]
					},
					{
						"id": 1,
						"name": "public-pipeline",
						"paused": true,
						"public": true,
						"archived": false,
						"team_name": "main",
						"last_updated": 1,
						"groups": [
							{
								"name": "group2",
								"jobs": ["job3", "job4"],
								"resources": ["resource3", "resource4"]
							}
						],
						"display": {
							"background_image": "background.jpg"
						}
					}
				]`))
			})

			It("returns all team's pipelines", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				var pipelines []map[string]any
				json.Unmarshal(body, &pipelines)

				Expect(pipelines).To(ConsistOf(
					HaveKeyWithValue("id", BeNumerically("==", publicPipeline.ID())),
					HaveKeyWithValue("id", BeNumerically("==", privatePipeline.ID())),
				))
			})

			Context("when the call to get active pipelines fails", func() {
				BeforeEach(func() {
					fakeTeam.PipelinesReturns(nil, errors.New("disaster"))
				})

				It("returns 500 internal server error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when authenticated as another team", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("returns only team's public pipelines", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				var pipelines []map[string]any
				json.Unmarshal(body, &pipelines)

				Expect(pipelines).To(ConsistOf(
					HaveKeyWithValue("id", BeNumerically("==", publicPipeline.ID())),
				))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			It("returns only team's public pipelines", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())
				var pipelines []map[string]any
				json.Unmarshal(body, &pipelines)

				Expect(pipelines).To(ConsistOf(
					HaveKeyWithValue("id", BeNumerically("==", publicPipeline.ID())),
				))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name", func() {
		var response *http.Response
		var fakePipeline *dbfakes.FakePipeline

		BeforeEach(func() {
			fakePipeline = new(dbfakes.FakePipeline)
			fakePipeline.IDReturns(4)
			fakePipeline.NameReturns("some-specific-pipeline")
			fakePipeline.PausedReturns(false)
			fakePipeline.PublicReturns(true)
			fakePipeline.TeamNameReturns("a-team")
			fakePipeline.GroupsReturns(atc.GroupConfigs{
				{
					Name:      "group1",
					Jobs:      []string{"job1", "job2"},
					Resources: []string{"resource1", "resource2"},
				},
				{
					Name:      "group2",
					Jobs:      []string{"job3", "job4"},
					Resources: []string{"resource3", "resource4"},
				},
			})
			fakePipeline.DisplayReturns(&atc.DisplayConfig{
				BackgroundImage: "background.jpg",
			})
			fakePipeline.LastUpdatedReturns(time.Unix(1, 0))
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipelines/some-specific-pipeline", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		Context("when authenticated as requested team", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeTeam.PipelineReturns(fakePipeline, true, nil)
			})

			It("returns 200 ok", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns application/json", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type": "application/json",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			It("returns a pipeline JSON", func() {
				body, err := io.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`
					{
						"id": 4,
						"name": "some-specific-pipeline",
						"paused": false,
						"public": true,
						"archived": false,
						"team_name": "a-team",
						"last_updated": 1,
						"groups": [
							{
								"name": "group1",
								"jobs": ["job1", "job2"],
								"resources": ["resource1", "resource2"]
							},
							{
								"name": "group2",
								"jobs": ["job3", "job4"],
								"resources": ["resource3", "resource4"]
							}
						],
						"display": {
							"background_image": "background.jpg"
						}
					}`))
			})
		})

		Context("when authenticated as another team", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(false)

				fakeTeam.PipelineReturns(fakePipeline, true, nil)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakeTeam.PipelineReturns(fakePipeline, true, nil)
					fakePipeline.PublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when not authenticated at all", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
				dbTeam.PipelineReturns(fakePipeline, true, nil)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/badge", func() {
		var response *http.Response
		var jobWithNoBuilds, jobWithSucceededBuild, jobWithAbortedBuild, jobWithErroredBuild, jobWithFailedBuild *dbfakes.FakeJob

		BeforeEach(func() {
			dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			dbPipeline.NameReturns("some-pipeline")
			fakeTeam.PipelineReturns(dbPipeline, true, nil)

			jobWithNoBuilds = new(dbfakes.FakeJob)
			jobWithSucceededBuild = new(dbfakes.FakeJob)
			jobWithAbortedBuild = new(dbfakes.FakeJob)
			jobWithErroredBuild = new(dbfakes.FakeJob)
			jobWithFailedBuild = new(dbfakes.FakeJob)

			succeededBuild := new(dbfakes.FakeBuild)
			succeededBuild.StatusReturns(db.BuildStatusSucceeded)
			jobWithSucceededBuild.FinishedAndNextBuildReturns(succeededBuild, nil, nil)

			abortedBuild := new(dbfakes.FakeBuild)
			abortedBuild.StatusReturns(db.BuildStatusAborted)
			jobWithAbortedBuild.FinishedAndNextBuildReturns(abortedBuild, nil, nil)

			erroredBuild := new(dbfakes.FakeBuild)
			erroredBuild.StatusReturns(db.BuildStatusErrored)
			jobWithErroredBuild.FinishedAndNextBuildReturns(erroredBuild, nil, nil)

			failedBuild := new(dbfakes.FakeBuild)
			failedBuild.StatusReturns(db.BuildStatusFailed)
			jobWithFailedBuild.FinishedAndNextBuildReturns(failedBuild, nil, nil)
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/badge")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					dbPipeline.PublicReturns(false)
				})

				Context("when user is authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})
					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})

				Context("when user is not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					dbPipeline.PublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			It("returns 200 OK", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})

			It("returns Content-Type as image/svg+xml and disables caching", func() {
				expectedHeaderEntries := map[string]string{
					"Content-Type":  "image/svg+xml",
					"Cache-Control": "no-cache, no-store, must-revalidate",
					"Expires":       "0",
				}
				Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
			})

			Context("when the pipeline has no finished builds", func() {
				BeforeEach(func() {
					dbPipeline.JobsReturns([]db.Job{jobWithNoBuilds}, nil)
				})

				It("returns an unknown badge", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="98" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="98" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="#9f9f9f" d="M37 0h61v20H37z" />
      <path fill="url(#b)" d="M0 0h98v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="66.5" y="15" fill="#010101" fill-opacity=".3">unknown</text>
      <text x="66.5" y="14">unknown</text>
   </g>
</svg>`))
				})
			})

			Context("when the pipeline has a successful build", func() {
				BeforeEach(func() {
					dbPipeline.JobsReturns([]db.Job{jobWithNoBuilds, jobWithSucceededBuild}, nil)
				})

				It("returns a successful badge", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="88" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="88" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="#44cc11" d="M37 0h51v20H37z" />
      <path fill="url(#b)" d="M0 0h88v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="61.5" y="15" fill="#010101" fill-opacity=".3">passing</text>
      <text x="61.5" y="14">passing</text>
   </g>
</svg>`))
				})
			})

			Context("when the pipeline has an aborted build", func() {
				BeforeEach(func() {
					dbPipeline.JobsReturns([]db.Job{jobWithNoBuilds, jobWithSucceededBuild, jobWithAbortedBuild}, nil)
				})

				It("returns an aborted badge", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="90" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="90" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="#8f4b2d" d="M37 0h53v20H37z" />
      <path fill="url(#b)" d="M0 0h90v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="62.5" y="15" fill="#010101" fill-opacity=".3">aborted</text>
      <text x="62.5" y="14">aborted</text>
   </g>
</svg>`))
				})
			})

			Context("when the pipeline has an errored build", func() {
				BeforeEach(func() {
					dbPipeline.JobsReturns([]db.Job{jobWithNoBuilds, jobWithSucceededBuild, jobWithAbortedBuild, jobWithErroredBuild}, nil)
				})

				It("returns an errored badge", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="88" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="88" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="#fe7d37" d="M37 0h51v20H37z" />
      <path fill="url(#b)" d="M0 0h88v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="61.5" y="15" fill="#010101" fill-opacity=".3">errored</text>
      <text x="61.5" y="14">errored</text>
   </g>
</svg>`))
				})
			})

			Context("when the pipeline has a failed build", func() {
				BeforeEach(func() {
					dbPipeline.JobsReturns([]db.Job{jobWithNoBuilds, jobWithSucceededBuild, jobWithAbortedBuild, jobWithErroredBuild, jobWithFailedBuild}, nil)
				})

				It("returns a failed badge", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(string(body)).To(Equal(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="80" height="20">
   <linearGradient id="b" x2="0" y2="100%">
      <stop offset="0" stop-color="#bbb" stop-opacity=".1" />
      <stop offset="1" stop-opacity=".1" />
   </linearGradient>
   <mask id="a">
      <rect width="80" height="20" rx="3" fill="#fff" />
   </mask>
   <g mask="url(#a)">
      <path fill="#555" d="M0 0h37v20H0z" />
      <path fill="#e05d44" d="M37 0h43v20H37z" />
      <path fill="url(#b)" d="M0 0h80v20H0z" />
   </g>
   <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
      <text x="57.5" y="15" fill="#010101" fill-opacity=".3">failing</text>
      <text x="57.5" y="14">failing</text>
   </g>
</svg>`))
				})
			})
		})
	})

	Describe("DELETE /api/v1/teams/:team_name/pipelines/:pipeline_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			pipelineName := "a-pipeline-name"
			req, err := http.NewRequest("DELETE", server.URL+"/api/v1/teams/a-team/pipelines/"+pipelineName, nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)

					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					dbPipeline.NameReturns("a-pipeline-name")
					fakeTeam.PipelineReturns(dbPipeline, true, nil)
				})

				It("returns 204 No Content", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNoContent))
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("injects the proper pipelineDB", func() {
					pipelineRef := fakeTeam.PipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "a-pipeline-name"}))
				})

				It("deletes the named pipeline from the database", func() {
					Expect(dbPipeline.DestroyCallCount()).To(Equal(1))
				})

				Context("when an error occurs destroying the pipeline", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						err := errors.New("disaster!")
						dbPipeline.DestroyReturns(err)
					})

					It("returns a 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when the user is not logged in", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("injects the proper pipelineDB", func() {
					pipelineRef := fakeTeam.PipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "a-pipeline"}))
				})

				Context("when pausing the pipeline succeeds", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.PauseReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when pausing the pipeline fails", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.PauseReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/archive", func() {
		var response *http.Response

		BeforeEach(func() {
			fakeAccess.IsAuthenticatedReturns(true)
			fakeAccess.IsAuthorizedReturns(true)
			dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
			fakeTeam.PipelineReturns(dbPipeline, true, nil)
		})

		JustBeforeEach(func() {
			request, _ := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/archive", nil)
			var err error
			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("archives the pipeline", func() {
			Expect(dbPipeline.ArchiveCallCount()).To(Equal(1), "Archive() called the wrong number of times")
		})

		Context("when archiving the pipeline fails due to the DB", func() {
			BeforeEach(func() {
				dbPipeline.ArchiveReturns(errors.New("pq: a db error"))
			})

			It("gives a server error", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})
			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)

					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					fakeTeam.PipelineReturns(dbPipeline, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("injects the proper pipelineDB", func() {
					pipelineRef := fakeTeam.PipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "a-pipeline"}))
				})

				Context("when unpausing the pipeline succeeds", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.UnpauseReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("notifies the resource scanner", func() {
						Expect(dbTeamFactory.NotifyResourceScannerCallCount()).To(Equal(1))
					})
				})

				Context("when unpausing the pipeline fails for an unknown reason", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.UnpauseReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/expose", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/expose", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("injects the proper pipelineDB", func() {
					Expect(fakeTeam.PipelineCallCount()).To(Equal(1))
					pipelineRef := fakeTeam.PipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "a-pipeline"}))
				})

				Context("when exposing the pipeline succeeds", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.ExposeReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when exposing the pipeline fails", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.ExposeReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/hide", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/hide", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})
			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("injects the proper pipeline", func() {
					pipelineRef := fakeTeam.PipelineArgsForCall(0)
					Expect(pipelineRef).To(Equal(atc.PipelineRef{Name: "a-pipeline"}))
				})

				Context("when hiding the pipeline succeeds", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.HideReturns(nil)
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when hiding the pipeline fails", func() {
					BeforeEach(func() {
						fakeTeam.PipelineReturns(dbPipeline, true, nil)
						dbPipeline.HideReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/ordering", func() {
		var response *http.Response
		var pipelineNames []string

		BeforeEach(func() {
			pipelineNames = []string{
				"a-pipeline",
				"another-pipeline",
				"yet-another-pipeline",
				"one-final-pipeline",
				"just-kidding",
			}
		})

		JustBeforeEach(func() {
			requestPayload, err := json.Marshal(pipelineNames)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/ordering", bytes.NewBuffer(requestPayload))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				Context("when ordering the pipelines succeeds", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesReturns(nil)
					})

					It("orders the pipelines", func() {
						Expect(fakeTeam.OrderPipelinesCallCount()).To(Equal(1))
						Expect(fakeTeam.OrderPipelinesArgsForCall(0)).To(Equal(pipelineNames))
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when a pipeline does not exist", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesReturns(db.ErrPipelineNotFound{Name: "a-pipeline"})
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(io.ReadAll(response.Body)).To(ContainSubstring("pipeline 'a-pipeline' not found"))
					})
				})

				Context("when ordering the pipelines fails", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/ordering", func() {
		var response *http.Response
		var instanceVars []atc.InstanceVars

		BeforeEach(func() {
			instanceVars = []atc.InstanceVars{
				{"branch": "test"},
				{},
				{"branch": "test-2"},
			}
		})

		JustBeforeEach(func() {
			requestPayload, err := json.Marshal(instanceVars)
			Expect(err).NotTo(HaveOccurred())

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/ordering", bytes.NewBuffer(requestPayload))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when requester belongs to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				})

				It("constructs team with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				Context("when ordering the pipelines succeeds", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesWithinGroupReturns(nil)
					})

					It("orders the pipelines", func() {
						Expect(fakeTeam.OrderPipelinesWithinGroupCallCount()).To(Equal(1))
						groupName, actualInstanceVars := fakeTeam.OrderPipelinesWithinGroupArgsForCall(0)
						Expect(groupName).To(Equal("a-pipeline"))
						Expect(actualInstanceVars).To(Equal(instanceVars))
					})

					It("returns 200", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})

				Context("when a pipeline does not exist", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesWithinGroupReturns(db.ErrPipelineNotFound{Name: "a-pipeline"})
					})

					It("returns 400", func() {
						Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(io.ReadAll(response.Body)).To(ContainSubstring("pipeline 'a-pipeline' not found"))
					})
				})

				Context("when ordering the pipelines fails", func() {
					BeforeEach(func() {
						fakeTeam.OrderPipelinesWithinGroupReturns(errors.New("welp"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/versions-db", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("GET", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/versions-db", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
				dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
				fakeTeam.PipelineReturns(dbPipeline, true, nil)
			})

			Context("when getting the debug versions db works", func() {
				BeforeEach(func() {
					scopeID := 789

					dbPipeline.LoadDebugVersionsDBReturns(
						&atc.DebugVersionsDB{
							ResourceVersions: []atc.DebugResourceVersion{
								{
									VersionID:  73,
									ResourceID: 127,
									CheckOrder: 123,
									ScopeID:    111,
								},
							},
							BuildOutputs: []atc.DebugBuildOutput{
								{
									DebugResourceVersion: atc.DebugResourceVersion{
										VersionID:  73,
										ResourceID: 127,
										CheckOrder: 123,
										ScopeID:    111,
									},
									BuildID: 66,
									JobID:   13,
								},
							},
							BuildInputs: []atc.DebugBuildInput{
								{
									DebugResourceVersion: atc.DebugResourceVersion{
										VersionID:  66,
										ResourceID: 77,
										CheckOrder: 88,
										ScopeID:    222,
									},
									BuildID:   66,
									JobID:     13,
									InputName: "some-input-name",
								},
							},
							BuildReruns: []atc.DebugBuildRerun{
								{
									JobID:   13,
									BuildID: 111,
									RerunOf: 222,
								},
							},
							Jobs: []atc.DebugJob{
								{
									ID:   13,
									Name: "bad-luck-job",
								},
							},
							Resources: []atc.DebugResource{
								{
									ID:      127,
									Name:    "resource-127",
									ScopeID: nil,
								},
								{
									ID:      128,
									Name:    "resource-128",
									ScopeID: &scopeID,
								},
							},
						},
						nil,
					)
				})

				It("constructs teamDB with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns application/json", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns a json representation of all the versions in the pipeline", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`{
				"ResourceVersions": [
					{
						"VersionID": 73,
						"ResourceID": 127,
						"CheckOrder": 123,
						"ScopeID": 111
			    }
				],
				"BuildOutputs": [
					{
						"VersionID": 73,
						"ResourceID": 127,
						"BuildID": 66,
						"JobID": 13,
						"CheckOrder": 123,
						"ScopeID": 111
					}
				],
				"BuildInputs": [
					{
						"VersionID": 66,
						"ResourceID": 77,
						"BuildID": 66,
						"JobID": 13,
						"CheckOrder": 88,
						"ScopeID": 222,
						"InputName": "some-input-name"
					}
				],
				"BuildReruns": [
					{
						"JobID": 13,
						"BuildID": 111,
						"RerunOf": 222
					}
				],
				"Jobs": [
					{
						"ID": 13,
						"Name": "bad-luck-job"
					}
				],
				"Resources": [
					{
						"ID": 127,
						"Name": "resource-127",
						"ScopeID": null
					},
					{
						"ID": 128,
						"Name": "resource-128",
						"ScopeID": 789
					}
				]
				}`))
				})
			})

			Context("when getting the debug versions db fails", func() {
				BeforeEach(func() {
					dbPipeline.LoadDebugVersionsDBReturns(nil, errors.New("nope"))
				})

				It("constructs teamDB with provided team name", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})

				It("does not return application/json", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "",
					}
					Expect(response).ShouldNot(IncludeHeaderEntries(expectedHeaderEntries))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/rename", func() {
		var response *http.Response
		var requestBody string

		BeforeEach(func() {
			requestBody = `{"name":"some-new-name"}`
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/rename", bytes.NewBufferString(requestBody))
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})
			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)

					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					fakeTeam.RenamePipelineReturns(true, nil)
				})

				It("finds the correct team", func() {
					Expect(dbTeamFactory.FindTeamCallCount()).To(Equal(1))
					Expect(dbTeamFactory.FindTeamArgsForCall(0)).To(Equal("a-team"))
				})

				It("renames the pipeline to the name provided", func() {
					Expect(fakeTeam.RenamePipelineCallCount()).To(Equal(1))
					oldName, newName := fakeTeam.RenamePipelineArgsForCall(0)
					Expect(oldName).To(Equal("a-pipeline"))
					Expect(newName).To(Equal("some-new-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the pipeline does not exist", func() {
					BeforeEach(func() {
						fakeTeam.RenamePipelineReturns(false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when renaming the pipeline errors", func() {
					BeforeEach(func() {
						fakeTeam.RenamePipelineReturns(false, errors.New("whoops"))
					})

					It("returns a 500 internal server error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the new name is an invalid identifier", func() {
					Context("and is a string", func() {
						BeforeEach(func() {
							requestBody = `{"name":"_some-new-name"}`
						})

						It("returns a warning in the response body", func() {
							Expect(io.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
									"invalid_identifier: pipeline: '_some-new-name' is not a valid identifier: must start with a lowercase letter or a number"
								]
							}`))
						})
					})
					Context("and is an empty string", func() {
						BeforeEach(func() {
							requestBody = `{"name":""}`
						})

						It("returns 400 Bad Request and an error in the response body", func() {
							Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
							Expect(io.ReadAll(response.Body)).To(MatchJSON(`
							{
								"errors": [
										"invalid_identifier: pipeline: identifier cannot be an empty string"
								]
							}`))
						})
					})
				})
			})

			Context("when requester does not belong to the team", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403 Forbidden", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401 Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/builds", func() {
		var response *http.Response
		var queryParams string

		JustBeforeEach(func() {
			var err error

			fakePipeline.NameReturns("some-pipeline")
			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when no params are passed", func() {
				It("does not set defaults for since and until", func() {
					Expect(fakePipeline.BuildsCallCount()).To(Equal(1))

					page := fakePipeline.BuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						Limit: 100,
					}))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?from=2&to=3&limit=8"
				})

				It("passes them through", func() {
					Expect(fakePipeline.BuildsCallCount()).To(Equal(1))

					page := fakePipeline.BuildsArgsForCall(0)
					Expect(page).To(Equal(db.Page{
						From:  db.NewIntPtr(2),
						To:    db.NewIntPtr(3),
						Limit: 8,
					}))
				})
			})

			Context("when getting the builds succeeds", func() {
				var returnedBuilds []db.BuildForAPI

				BeforeEach(func() {
					queryParams = "?since=5&limit=2"

					build1 := new(dbfakes.FakeBuildForAPI)
					build1.IDReturns(4)
					build1.NameReturns("2")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.TeamNameReturns("some-team")
					build1.StatusReturns(db.BuildStatusStarted)
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))

					build2 := new(dbfakes.FakeBuildForAPI)
					build2.IDReturns(2)
					build2.NameReturns("1")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.TeamNameReturns("some-team")
					build2.StatusReturns(db.BuildStatusSucceeded)
					build2.StartTimeReturns(time.Unix(101, 0))
					build2.EndTimeReturns(time.Unix(200, 0))

					returnedBuilds = []db.BuildForAPI{build1, build2}
					fakePipeline.BuildsReturns(returnedBuilds, db.Pagination{}, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					expectedHeaderEntries := map[string]string{
						"Content-Type": "application/json",
					}
					Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
				})

				It("returns the builds", func() {
					body, err := io.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
					{
						"id": 4,
						"name": "2",
						"job_name": "some-job",
						"status": "started",
						"api_url": "/api/v1/builds/4",
						"pipeline_name":"some-pipeline",
						"team_name": "some-team",
						"start_time": 1,
						"end_time": 100
					},
					{
						"id": 2,
						"name": "1",
						"job_name": "some-job",
						"status": "succeeded",
						"api_url": "/api/v1/builds/2",
						"pipeline_name": "some-pipeline",
						"team_name": "some-team",
						"start_time": 101,
						"end_time": 200
					}
				]`))
				})

				Context("when next/previous pages are available", func() {
					BeforeEach(func() {
						fakePipeline.BuildsReturns(returnedBuilds, db.Pagination{
							Newer: &db.Page{From: db.NewIntPtr(4), Limit: 2},
							Older: &db.Page{To: db.NewIntPtr(2), Limit: 2},
						}, nil)
					})

					It("returns Link headers per rfc5988", func() {
						Expect(response.Header["Link"]).To(ConsistOf([]string{
							fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/builds?from=4&limit=2>; rel="previous"`, externalURL),
							fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/builds?to=2&limit=2>; rel="next"`, externalURL),
						}))
					})

					Context("and pipeline is instanced", func() {
						BeforeEach(func() {
							fakePipeline.InstanceVarsReturns(atc.InstanceVars{"branch": "master"})
						})

						It("returns Link headers per rfc5988", func() {
							link := fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/builds?`, externalURL)
							Expect(response.Header["Link"]).To(ConsistOf([]string{
								link + `to=2&limit=2&vars.branch=%22master%22>; rel="next"`,
								link + `from=4&limit=2&vars.branch=%22master%22>; rel="previous"`,
							}))
						})
					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					fakePipeline.BuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/builds", func() {
		var plan atc.Plan
		var response *http.Response

		BeforeEach(func() {
			plan = atc.Plan{
				Task: &atc.TaskPlan{
					Config: &atc.TaskConfig{
						Run: atc.TaskRunConfig{
							Path: "ls",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			reqPayload, err := json.Marshal(plan)
			Expect(err).NotTo(HaveOccurred())

			req, err := http.NewRequest("POST", server.URL+"/api/v1/teams/a-team/pipelines/a-pipeline/builds", bytes.NewBuffer(reqPayload))
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})

			It("does not trigger a build", func() {
				Expect(dbPipeline.CreateOneOffBuildCallCount()).To(BeZero())
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
				})

				It("returns 403", func() {
					Expect(response.StatusCode).To(Equal(http.StatusForbidden))
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					dbTeamFactory.FindTeamReturns(fakeTeam, true, nil)
					fakeTeam.PipelineReturns(dbPipeline, true, nil)
				})

				Context("when creating a started build fails", func() {
					BeforeEach(func() {
						dbPipeline.CreateStartedBuildReturns(nil, errors.New("oh no!"))
					})

					It("returns 500 Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when creating a started build succeeds", func() {
					var fakeBuild *dbfakes.FakeBuild

					BeforeEach(func() {
						fakeBuild = new(dbfakes.FakeBuild)
						fakeBuild.IDReturns(42)
						fakeBuild.NameReturns("1")
						fakeBuild.TeamNameReturns("some-team")
						fakeBuild.StatusReturns("started")
						fakeBuild.StartTimeReturns(time.Unix(1, 0))
						fakeBuild.EndTimeReturns(time.Unix(100, 0))
						fakeBuild.ReapTimeReturns(time.Unix(200, 0))

						dbPipeline.CreateStartedBuildReturns(fakeBuild, nil)
					})

					It("returns 201 Created", func() {
						Expect(response.StatusCode).To(Equal(http.StatusCreated))
					})

					It("returns Content-Type 'application/json'", func() {
						expectedHeaderEntries := map[string]string{
							"Content-Type": "application/json",
						}
						Expect(response).Should(IncludeHeaderEntries(expectedHeaderEntries))
					})

					It("creates a started build", func() {
						Expect(dbPipeline.CreateStartedBuildCallCount()).To(Equal(1))
						Expect(dbPipeline.CreateStartedBuildArgsForCall(0)).To(Equal(plan))
					})

					It("returns the created build", func() {
						body, err := io.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
								"id": 42,
								"name": "1",
								"team_name": "some-team",
								"status": "started",
								"api_url": "/api/v1/builds/42",
								"start_time": 1,
								"end_time": 100,
								"reap_time": 200
						}`))
					})
				})
			})
		})
	})
})
