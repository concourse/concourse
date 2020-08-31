package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Jobs API", func() {
	var fakeJob *dbfakes.FakeJob
	var versionedResourceTypes atc.VersionedResourceTypes
	var fakePipeline *dbfakes.FakePipeline

	BeforeEach(func() {
		fakeJob = new(dbfakes.FakeJob)
		fakePipeline = new(dbfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)

		versionedResourceTypes = atc.VersionedResourceTypes{
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-1",
					Type:   "some-base-type-1",
					Source: atc.Source{"some": "source-1"},
				},
				Version: atc.Version{"some": "version-1"},
			},
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-2",
					Type:   "some-base-type-2",
					Source: atc.Source{"some": "source-2"},
				},
				Version: atc.Version{"some": "version-2"},
			},
			atc.VersionedResourceType{
				ResourceType: atc.ResourceType{
					Name:   "some-resource-3",
					Type:   "some-base-type-3",
					Source: atc.Source{"some": "source-3"},
				},
				Version: atc.Version{"some": "version-3"},
			},
		}

		fakePipeline.ResourceTypesReturns([]db.ResourceType{
			fakeDBResourceType(versionedResourceTypes[0]),
			fakeDBResourceType(versionedResourceTypes[1]),
			fakeDBResourceType(versionedResourceTypes[2]),
		}, nil)
	})

	Describe("GET /api/v1/jobs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/jobs", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			dbJobFactory.VisibleJobsReturns(atc.Dashboard{
				atc.DashboardJob{
					ID:           1,
					Name:         "some-job",
					Paused:       true,
					PipelineName: "some-pipeline",
					TeamName:     "some-team",

					Inputs: []atc.DashboardJobInput{
						{
							Name:     "some-input",
							Resource: "some-input",
							Trigger:  false,
						},
						{
							Name:     "some-name",
							Resource: "some-other-input",
							Passed:   []string{"a", "b"},
							Trigger:  true,
						},
					},

					NextBuild: &atc.DashboardBuild{
						ID:           3,
						Name:         "2",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
						Status:       "started",
					},
					FinishedBuild: &atc.DashboardBuild{
						ID:           1,
						Name:         "1",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						TeamName:     "some-team",
						Status:       "succeeded",
						StartTime:    time.Unix(1, 0),
						EndTime:      time.Unix(100, 0),
					},

					Groups: []string{"group-1", "group-2"},
				},
			}, nil)
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

		It("returns all jobs from public pipelines and pipelines in authenticated teams", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`[
			{
				"id": 1,
				"name": "some-job",
				"pipeline_name": "some-pipeline",
				"team_name": "some-team",
				"paused": true,
				"next_build": {
					"id": 3,
					"team_name": "some-team",
					"name": "2",
					"status": "started",
					"job_name": "some-job",
					"api_url": "/api/v1/builds/3",
					"pipeline_name": "some-pipeline"
				},
				"finished_build": {
					"id": 1,
					"team_name": "some-team",
					"name": "1",
					"status": "succeeded",
					"job_name": "some-job",
					"api_url": "/api/v1/builds/1",
					"pipeline_name": "some-pipeline",
					"start_time": 1,
					"end_time": 100
				},
				"inputs": [
					{
						"name": "some-input",
						"resource": "some-input",
						"trigger": false
					},
					{
						"name": "some-name",
						"resource": "some-other-input",
						"passed": [
							"a",
							"b"
						],
						"trigger": true
					}
				],
				"groups": ["group-1", "group-2"]
			}
			]`))
		})

		Context("when getting the jobs fails", func() {
			BeforeEach(func() {
				dbJobFactory.VisibleJobsReturns(nil, errors.New("nope"))
			})

			It("returns 500", func() {
				Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when there are no visible jobs", func() {
			BeforeEach(func() {
				dbJobFactory.VisibleJobsReturns(nil, nil)
			})

			It("returns empty array", func() {
				body, err := ioutil.ReadAll(response.Body)
				Expect(err).NotTo(HaveOccurred())

				Expect(body).To(MatchJSON(`[]`))
			})
		})

		Context("when not authenticated", func() {
			It("populates job factory with no team names", func() {
				Expect(dbJobFactory.VisibleJobsCallCount()).To(Equal(1))
				Expect(dbJobFactory.VisibleJobsArgsForCall(0)).To(BeEmpty())
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeAccess.TeamNamesReturns([]string{"some-team"})
			})

			It("constructs job factory with provided team names", func() {
				Expect(dbJobFactory.VisibleJobsCallCount()).To(Equal(1))
				Expect(dbJobFactory.VisibleJobsArgsForCall(0)).To(ContainElement("some-team"))
			})

			Context("user has the admin privilege", func() {
				BeforeEach(func() {
					fakeAccess.IsAdminReturns(true)
				})

				It("returns all jobs from public and private pipelines from unauthenticated teams", func() {
					Expect(dbJobFactory.AllActiveJobsCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job")
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
					fakePipeline.JobReturns(fakeJob, true, nil)
					fakePipeline.PublicReturns(true)
					fakeJob.FinishedAndNextBuildReturns(nil, nil, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authenticated and not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(false)
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
					fakePipeline.JobReturns(fakeJob, true, nil)
					fakePipeline.PublicReturns(true)
					fakeJob.FinishedAndNextBuildReturns(nil, nil, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authenticated and authorized", func() {
			var build1 *dbfakes.FakeBuild
			var build2 *dbfakes.FakeBuild

			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when getting the build succeeds", func() {
				BeforeEach(func() {
					build1 = new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.TeamNameReturns("some-team")
					build1.StatusReturns(db.BuildStatusSucceeded)
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))

					build2 = new(dbfakes.FakeBuild)
					build2.IDReturns(3)
					build2.NameReturns("2")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.TeamNameReturns("some-team")
					build2.StatusReturns(db.BuildStatusStarted)
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						fakeJob.IDReturns(1)
						fakeJob.PausedReturns(true)
						fakeJob.FirstLoggedBuildIDReturns(99)
						fakeJob.PipelineNameReturns("some-pipeline")
						fakeJob.NameReturns("some-job")
						fakeJob.TagsReturns([]string{"group-1", "group-2"})
						fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)

						fakePipeline.JobReturns(fakeJob, true, nil)
					})

					It("fetches the inputs", func() {
						Expect(fakeJob.InputsCallCount()).To(Equal(1))
					})

					Context("when getting the inputs fails", func() {
						BeforeEach(func() {
							fakeJob.InputsReturns(nil, errors.New("nope"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when getting the inputs succeeds", func() {
						BeforeEach(func() {
							fakeJob.InputsReturns([]atc.JobInput{
								{
									Name:     "some-input",
									Resource: "some-input",
								},
								{
									Name:     "some-name",
									Resource: "some-other-input",
									Passed:   []string{"a", "b"},
									Trigger:  true,
								},
							}, nil)
						})

						It("fetches the outputs", func() {
							Expect(fakeJob.OutputsCallCount()).To(Equal(1))
						})

						Context("when getting the outputs fails", func() {
							BeforeEach(func() {
								fakeJob.OutputsReturns(nil, errors.New("nope"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("when getting the outputs succeeds", func() {
							BeforeEach(func() {
								fakeJob.OutputsReturns([]atc.JobOutput{
									{
										Name:     "some-output",
										Resource: "some-output",
									},
									{
										Name:     "some-other-output",
										Resource: "some-other-output",
									},
								}, nil)
							})

							It("fetches by job", func() {
								Expect(fakeJob.FinishedAndNextBuildCallCount()).To(Equal(1))
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

							It("returns the job's name, if it's paused, and any running and finished builds", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{
							"id": 1,
							"name": "some-job",
							"pipeline_name": "some-pipeline",
							"team_name": "some-team",
							"paused": true,
							"first_logged_build_id": 99,
							"next_build": {
								"id": 3,
								"name": "2",
								"job_name": "some-job",
								"status": "started",
								"api_url": "/api/v1/builds/3",
								"pipeline_name": "some-pipeline",
								"team_name": "some-team"
							},
							"finished_build": {
								"id": 1,
								"name": "1",
								"job_name": "some-job",
								"status": "succeeded",
								"api_url": "/api/v1/builds/1",
								"pipeline_name": "some-pipeline",
								"team_name": "some-team",
								"start_time": 1,
								"end_time": 100
							},
							"inputs": [
								{
									"name": "some-input",
									"resource": "some-input",
									"trigger": false
								},
								{
									"name": "some-name",
									"resource": "some-other-input",
									"passed": ["a", "b"],
									"trigger": true
								}
							],
							"outputs": [
								{
									"name": "some-output",
									"resource": "some-output"
								},
								{
									"name": "some-other-output",
									"resource": "some-other-output"
								}
							],
							"groups": ["group-1", "group-2"]
						}`))

							})

							Context("when there are no running or finished builds", func() {
								BeforeEach(func() {
									fakeJob.FinishedAndNextBuildReturns(nil, nil, nil)
								})

								It("returns null as their entries", func() {
									var job atc.Job
									err := json.NewDecoder(response.Body).Decode(&job)
									Expect(err).NotTo(HaveOccurred())

									Expect(job.NextBuild).To(BeNil())
									Expect(job.FinishedBuild).To(BeNil())
								})
							})

							Context("when getting the job's builds fails", func() {
								BeforeEach(func() {
									fakeJob.FinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
								})

								It("returns 500", func() {
									Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})
						})
					})
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/badge", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/badge")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated and not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(true)
				fakeAccess.IsAuthorizedReturns(false)
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
					fakePipeline.PublicReturns(true)
					fakePipeline.JobReturns(fakeJob, true, nil)
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

				fakePipeline.JobReturns(fakeJob, true, nil)
				fakeJob.NameReturns("some-job")
			})

			It("fetches by job", func() {
				Expect(fakeJob.FinishedAndNextBuildCallCount()).To(Equal(1))
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

			Context("when generates bagde title", func() {
				It("uses url `title` parameter", func() {
					response, err := client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/badge?title=cov")
					Expect(err).NotTo(HaveOccurred())

					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(strings.Contains(string(body), `
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">cov</text>
      <text x="18.5" y="14">cov</text>
`)).Should(BeTrue())
				})
				It("uses default `build` title if not specified", func() {
					response, err := client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/badge")
					Expect(err).NotTo(HaveOccurred())

					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(strings.Contains(string(body), `
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
`)).Should(BeTrue())
				})
				It("uses default `build` title if url `title` parameter is falsy", func() {
					response, err := client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/badge?title=")
					Expect(err).NotTo(HaveOccurred())

					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(strings.Contains(string(body), `
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">build</text>
      <text x="18.5" y="14">build</text>
`)).Should(BeTrue())
				})
				It("html escapes title", func() {
					response, err := client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/badge?title=%24cov")
					Expect(err).NotTo(HaveOccurred())

					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(strings.Contains(string(body), `
      <text x="18.5" y="15" fill="#010101" fill-opacity=".3">$cov</text>
      <text x="18.5" y="14">$cov</text>
`)).Should(BeTrue())
				})
			})

			Context("when the finished build is successful", func() {
				BeforeEach(func() {
					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))
					build1.StatusReturns(db.BuildStatusSucceeded)

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(3)
					build2.NameReturns("2")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.StatusReturns(db.BuildStatusStarted)

					fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns some SVG showing that the job is successful", func() {
					body, err := ioutil.ReadAll(response.Body)
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

			Context("when the finished build is failed", func() {
				BeforeEach(func() {
					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))
					build1.StatusReturns(db.BuildStatusFailed)

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(3)
					build2.NameReturns("2")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.StatusReturns(db.BuildStatusStarted)

					fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns some SVG showing that the job has failed", func() {
					body, err := ioutil.ReadAll(response.Body)
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

			Context("when the finished build was aborted", func() {
				BeforeEach(func() {
					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.StatusReturns(db.BuildStatusAborted)

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(1)
					build2.NameReturns("1")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.StartTimeReturns(time.Unix(1, 0))
					build2.EndTimeReturns(time.Unix(100, 0))
					build2.StatusReturns(db.BuildStatusAborted)

					fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns some SVG showing that the job was aborted", func() {
					body, err := ioutil.ReadAll(response.Body)
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

			Context("when the finished build errored", func() {
				BeforeEach(func() {
					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))
					build1.StatusReturns(db.BuildStatusErrored)

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(3)
					build2.NameReturns("2")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.StatusReturns(db.BuildStatusStarted)

					fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns some SVG showing that the job has errored", func() {
					body, err := ioutil.ReadAll(response.Body)
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

			Context("when there are no running or finished builds", func() {
				BeforeEach(func() {
					fakeJob.FinishedAndNextBuildReturns(nil, nil, nil)
				})

				It("returns an unknown badge", func() {
					body, err := ioutil.ReadAll(response.Body)
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

			Context("when getting the job's builds fails", func() {
				BeforeEach(func() {
					fakeJob.FinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not present", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", func() {
		var response *http.Response
		var dashboardResponse atc.Dashboard

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the dashboard succeeds", func() {

			BeforeEach(func() {

				dashboardResponse = atc.Dashboard{
					{
						ID:           1,
						Name:         "job-1",
						PipelineName: "another-pipeline",
						TeamName:     "some-team",
						Paused:       true,
						NextBuild: &atc.DashboardBuild{
							ID:           3,
							Name:         "2",
							JobName:      "job-1",
							PipelineName: "another-pipeline",
							TeamName:     "some-team",
							Status:       "started",
						},
						FinishedBuild: &atc.DashboardBuild{
							ID:           1,
							Name:         "1",
							JobName:      "job-1",
							PipelineName: "another-pipeline",
							TeamName:     "some-team",
							Status:       "succeeded",
							StartTime:    time.Unix(1, 0),
							EndTime:      time.Unix(100, 0),
						},
						TransitionBuild: &atc.DashboardBuild{
							ID:           5,
							Name:         "five",
							JobName:      "job-1",
							PipelineName: "another-pipeline",
							TeamName:     "some-team",
							Status:       "failed",
							StartTime:    time.Unix(101, 0),
							EndTime:      time.Unix(200, 0),
						},
						Inputs: []atc.DashboardJobInput{
							{
								Name:     "input-1",
								Resource: "input-1",
							},
						},
						Groups: []string{
							"group-1", "group-2",
						},
					},
					{
						ID:           2,
						Name:         "job-2",
						PipelineName: "another-pipeline",
						TeamName:     "some-team",
						Paused:       true,
						NextBuild:    nil,
						FinishedBuild: &atc.DashboardBuild{
							ID:           4,
							Name:         "1",
							JobName:      "job-2",
							PipelineName: "another-pipeline",
							TeamName:     "some-team",
							Status:       "succeeded",
							StartTime:    time.Unix(101, 0),
							EndTime:      time.Unix(200, 0),
						},
						TransitionBuild: nil,
						Inputs: []atc.DashboardJobInput{
							{
								Name:     "input-2",
								Resource: "input-2",
							},
						},
						Groups: []string{
							"group-2",
						},
					},
					{
						ID:              3,
						Name:            "job-3",
						PipelineName:    "another-pipeline",
						TeamName:        "some-team",
						Paused:          true,
						NextBuild:       nil,
						FinishedBuild:   nil,
						TransitionBuild: nil,
						Inputs: []atc.DashboardJobInput{
							{
								Name:     "input-3",
								Resource: "input-3",
							},
						},
						Groups: []string{},
					},
				}
				fakePipeline.DashboardReturns(dashboardResponse, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(false)
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
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthorizedReturns(true)
					fakeAccess.IsAuthenticatedReturns(true)
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

				It("returns each job's name and any running and finished builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
							{
								"id": 1,
								"name": "job-1",
								"pipeline_name": "another-pipeline",
								"team_name": "some-team",
								"paused": true,
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"api_url": "/api/v1/builds/3",
									"pipeline_name": "another-pipeline",
									"team_name": "some-team"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"api_url": "/api/v1/builds/1",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team",
									"start_time": 1,
									"end_time": 100
								},
								"transition_build": {
									"id": 5,
									"name": "five",
									"job_name": "job-1",
									"status": "failed",
									"api_url": "/api/v1/builds/5",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team",
									"start_time": 101,
									"end_time": 200
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": false}],
								"groups": ["group-1", "group-2"]
							},
							{
								"id": 2,
								"name": "job-2",
								"pipeline_name": "another-pipeline",
								"team_name": "some-team",
								"paused": true,
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"api_url": "/api/v1/builds/4",
									"pipeline_name": "another-pipeline",
									"team_name": "some-team",
									"start_time": 101,
									"end_time": 200
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": false}],
								"groups": ["group-2"]
							},
							{
								"id": 3,
								"name": "job-3",
								"pipeline_name": "another-pipeline",
								"team_name": "some-team",
								"paused": true,
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": false}],
								"groups": []
							}
						]`))
				})

				Context("when there are no jobs in dashboard", func() {
					BeforeEach(func() {
						dashboardResponse = atc.Dashboard{}
						fakePipeline.DashboardReturns(dashboardResponse, nil)
					})
					It("should return an empty array", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[]`))
					})
				})

				Context("when getting the dashboard fails", func() {
					Context("with an unknown error", func() {
						BeforeEach(func() {
							fakePipeline.DashboardReturns(nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", func() {
		var response *http.Response
		var queryParams string

		JustBeforeEach(func() {
			var err error

			fakePipeline.NameReturns("some-pipeline")
			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated and not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
				fakeAccess.IsAuthenticatedReturns(true)
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
					fakeBuild := new(dbfakes.FakeBuild)

					fakePipeline.PublicReturns(true)
					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.BuildReturns(fakeBuild, true, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when getting the job succeeds", func() {
				BeforeEach(func() {
					fakeJob.NameReturns("some-job")
					fakePipeline.JobReturns(fakeJob, true, nil)
				})

				Context("when no params are passed", func() {
					It("does not set defaults for since and until", func() {
						Expect(fakeJob.BuildsCallCount()).To(Equal(1))

						page := fakeJob.BuildsArgsForCall(0)
						Expect(page).To(Equal(db.Page{
							From:  0,
							To:    0,
							Limit: 100,
						}))
					})
				})

				Context("when all the params are passed", func() {
					BeforeEach(func() {
						queryParams = "?from=2&to=3&limit=8"
					})

					It("passes them through", func() {
						Expect(fakeJob.BuildsCallCount()).To(Equal(1))

						page := fakeJob.BuildsArgsForCall(0)
						Expect(page).To(Equal(db.Page{
							From:  2,
							To:    3,
							Limit: 8,
						}))
					})
				})

				Context("when getting the builds succeeds", func() {
					var returnedBuilds []db.Build

					BeforeEach(func() {
						queryParams = "?since=5&limit=2"

						build1 := new(dbfakes.FakeBuild)
						build1.IDReturns(4)
						build1.NameReturns("2")
						build1.JobNameReturns("some-job")
						build1.PipelineNameReturns("some-pipeline")
						build1.TeamNameReturns("some-team")
						build1.StatusReturns(db.BuildStatusStarted)
						build1.StartTimeReturns(time.Unix(1, 0))
						build1.EndTimeReturns(time.Unix(100, 0))

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(2)
						build2.NameReturns("1")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("some-pipeline")
						build2.TeamNameReturns("some-team")
						build2.StatusReturns(db.BuildStatusSucceeded)
						build2.StartTimeReturns(time.Unix(101, 0))
						build2.EndTimeReturns(time.Unix(200, 0))

						build3 := new(dbfakes.FakeBuild)
						build3.IDReturns(3)
						build3.NameReturns("1.1")
						build3.JobNameReturns("some-job")
						build3.PipelineNameReturns("some-pipeline")
						build3.TeamNameReturns("some-team")
						build3.StatusReturns(db.BuildStatusSucceeded)
						build3.StartTimeReturns(time.Unix(102, 0))
						build3.EndTimeReturns(time.Unix(300, 0))
						build3.RerunOfReturns(2)
						build3.RerunOfNameReturns("1")
						build3.RerunNumberReturns(3)

						returnedBuilds = []db.Build{build1, build2, build3}
						fakeJob.BuildsReturns(returnedBuilds, db.Pagination{}, nil)
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
						body, err := ioutil.ReadAll(response.Body)
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
					},
					{
						"id": 3,
						"name": "1.1",
						"job_name": "some-job",
						"status": "succeeded",
						"api_url": "/api/v1/builds/3",
						"pipeline_name": "some-pipeline",
						"team_name": "some-team",
						"start_time": 102,
						"end_time": 300,
						"rerun_of": {
							"id": 2,
							"name": "1"
						},
						"rerun_number": 3
					}
				]`))
					})

					Context("when next/previous pages are available", func() {
						BeforeEach(func() {
							fakeJob.BuildsReturns(returnedBuilds, db.Pagination{
								Newer: &db.Page{From: 4, Limit: 2},
								Older: &db.Page{To: 2, Limit: 2},
							}, nil)
						})

						It("returns Link headers per rfc5988", func() {
							Expect(response.Header["Link"]).To(ConsistOf([]string{
								fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?from=4&limit=2>; rel="previous"`, externalURL),
								fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?to=2&limit=2>; rel="next"`, externalURL),
							}))
						})
					})
				})

				Context("when getting the build fails", func() {
					BeforeEach(func() {
						fakeJob.BuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
					})

					It("returns 404 Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when getting the job fails", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized and authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when getting the job fails", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, errors.New("errorrr"))
				})

				It("returns a 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns a 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when getting the job succeeds", func() {
				BeforeEach(func() {
					fakeJob.NameReturns("some-job")
					fakePipeline.JobReturns(fakeJob, true, nil)
				})

				Context("when manual triggering is disabled", func() {
					BeforeEach(func() {
						fakeJob.DisableManualTriggerReturns(true)
					})

					It("should return 409", func() {
						Expect(response.StatusCode).To(Equal(http.StatusConflict))
					})

					It("does not trigger the build", func() {
						Expect(fakeJob.CreateBuildCallCount()).To(Equal(0))
					})
				})

				Context("when manual triggering is enabled", func() {
					BeforeEach(func() {
						fakeJob.DisableManualTriggerReturns(false)
					})

					Context("when triggering the build fails", func() {
						BeforeEach(func() {
							fakeJob.CreateBuildReturns(nil, errors.New("nopers"))
						})
						It("returns a 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when triggering the build succeeds", func() {
						BeforeEach(func() {
							build := new(dbfakes.FakeBuild)
							build.IDReturns(42)
							build.NameReturns("1")
							build.JobNameReturns("some-job")
							build.PipelineNameReturns("a-pipeline")
							build.TeamNameReturns("some-team")
							build.StatusReturns(db.BuildStatusStarted)
							build.StartTimeReturns(time.Unix(1, 0))
							build.EndTimeReturns(time.Unix(100, 0))

							fakeJob.CreateBuildReturns(build, nil)
						})

						It("triggers the build", func() {
							Expect(fakeJob.CreateBuildCallCount()).To(Equal(1))
						})

						Context("when finding the pipeline resources fails", func() {
							BeforeEach(func() {
								fakePipeline.ResourcesReturns(nil, errors.New("nope"))
							})

							It("returns a 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("when finding the pipeline resources succeeds", func() {
							var fakeResource *dbfakes.FakeResource

							BeforeEach(func() {
								fakeResource = new(dbfakes.FakeResource)
								fakeResource.NameReturns("some-input")
								fakeResource.CurrentPinnedVersionReturns(atc.Version{"some": "version"})

								fakePipeline.ResourcesReturns([]db.Resource{fakeResource}, nil)
							})

							Context("when finding the pipeline resource types fails", func() {
								BeforeEach(func() {
									fakePipeline.ResourceTypesReturns(nil, errors.New("nope"))
								})

								It("returns a 500", func() {
									Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})

							Context("when finding the pipeline resources types succeeds", func() {
								var fakeResourceType *dbfakes.FakeResourceType

								BeforeEach(func() {
									fakeResourceType = new(dbfakes.FakeResourceType)
									fakeResourceType.NameReturns("some-input")

									fakePipeline.ResourceTypesReturns([]db.ResourceType{fakeResourceType}, nil)
								})

								It("fetches the job inputs", func() {
									Expect(fakeJob.InputsCallCount()).To(Equal(1))
								})

								Context("when it fails to fetch the job inputs", func() {
									BeforeEach(func() {
										fakeJob.InputsReturns(nil, errors.New("nope"))
									})

									It("returns a 500", func() {
										Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
									})
								})

								Context("when the job inputs are successfully fetched", func() {
									BeforeEach(func() {
										fakeJob.InputsReturns([]atc.JobInput{
											{
												Name:     "some-input",
												Resource: "some-input",
											},
										}, nil)
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

									It("creates a check for the resource", func() {
										Expect(dbCheckFactory.TryCreateCheckCallCount()).To(Equal(1))
									})

									It("runs the check from the current pinned version", func() {
										_, _, _, fromVersion, _ := dbCheckFactory.TryCreateCheckArgsForCall(0)
										Expect(fromVersion).To(Equal(atc.Version{"some": "version"}))
									})

									It("notifies the checker to run", func() {
										Expect(dbCheckFactory.NotifyCheckerCallCount()).To(Equal(1))
									})

									It("returns the build", func() {
										body, err := ioutil.ReadAll(response.Body)
										Expect(err).NotTo(HaveOccurred())

										Expect(body).To(MatchJSON(`{
							"id": 42,
							"name": "1",
							"job_name": "some-job",
							"status": "started",
							"api_url": "/api/v1/builds/42",
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"start_time": 1,
							"end_time": 100
						}`))
									})
								})
							})
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/inputs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/inputs")
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
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("some-error"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, nil)
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(fakeJob, true, nil)
					})

					Context("when getting the resources fails", func() {
						BeforeEach(func() {
							fakePipeline.ResourcesReturns(nil, errors.New("some-error"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when getting the resources succeeds", func() {
						BeforeEach(func() {
							resource1 := new(dbfakes.FakeResource)
							resource1.IDReturns(1)
							resource1.NameReturns("some-resource")
							resource1.TypeReturns("some-type")
							resource1.SourceReturns(atc.Source{"some": "source"})

							resource2 := new(dbfakes.FakeResource)
							resource1.IDReturns(2)
							resource2.NameReturns("some-other-resource")
							resource2.TypeReturns("some-other-type")
							resource2.SourceReturns(atc.Source{"some": "other-source"})

							fakePipeline.ResourcesReturns([]db.Resource{resource1, resource2}, nil)
						})

						Context("when getting the input versions for the job fails", func() {
							BeforeEach(func() {
								fakeJob.GetFullNextBuildInputsReturns(nil, false, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("when the job has no input versions available", func() {
							BeforeEach(func() {
								fakeJob.GetFullNextBuildInputsReturns(nil, false, nil)
							})

							It("returns 404", func() {
								Expect(response.StatusCode).To(Equal(http.StatusNotFound))
							})
						})

						Context("when the job has input versions", func() {
							BeforeEach(func() {
								inputs := []db.BuildInput{
									{
										Name:       "some-input",
										Version:    atc.Version{"some": "version"},
										ResourceID: 1,
									},
									{
										Name:       "some-other-input",
										Version:    atc.Version{"some": "other-version"},
										ResourceID: 2,
									},
								}

								fakeJob.GetFullNextBuildInputsReturns(inputs, true, nil)
							})

							It("fetches the job config", func() {
								Expect(fakeJob.ConfigCallCount()).To(Equal(1))
							})

							Context("when it fails to fetch the job config", func() {
								BeforeEach(func() {
									fakeJob.ConfigReturns(atc.JobConfig{}, errors.New("nope"))
								})

								It("returns a 500", func() {
									Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
								})
							})

							Context("when the job inputs are successfully fetched", func() {
								BeforeEach(func() {
									fakeJob.ConfigReturns(atc.JobConfig{
										Name: "some-job",
										PlanSequence: []atc.Step{
											{
												Config: &atc.GetStep{
													Name:     "some-input",
													Resource: "some-resource",
													Passed:   []string{"job-a", "job-b"},
													Params:   atc.Params{"some": "params"},
												},
											},
											{
												Config: &atc.GetStep{
													Name:     "some-other-input",
													Resource: "some-other-resource",
													Passed:   []string{"job-c", "job-d"},
													Params:   atc.Params{"some": "other-params"},
													Tags:     []string{"some-tag"},
												},
											},
										},
									}, nil)
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

								It("returns the inputs", func() {
									body, err := ioutil.ReadAll(response.Body)
									Expect(err).NotTo(HaveOccurred())

									Expect(body).To(MatchJSON(`[
									{
										"name": "some-input",
										"resource": "some-resource",
										"type": "some-type",
										"source": {"some": "source"},
										"version": {"some": "version"},
										"params": {"some": "params"}
									},
									{
										"name": "some-other-input",
										"resource": "some-other-resource",
										"type": "some-other-type",
										"source": {"some": "other-source"},
										"version": {"some": "other-version"},
										"params": {"some": "other-params"},
										"tags": ["some-tag"]
									}
								]`))
								})
							})
						})
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/some-build")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when getting the job succeeds", func() {
				var fakeJob *dbfakes.FakeJob

				BeforeEach(func() {
					fakeJob = new(dbfakes.FakeJob)
					fakeJob.NameReturns("some-job")
					fakePipeline.JobReturns(fakeJob, true, nil)
				})

				Context("when getting the build succeeds", func() {
					BeforeEach(func() {
						dbBuild := new(dbfakes.FakeBuild)
						dbBuild.IDReturns(1)
						dbBuild.NameReturns("1")
						dbBuild.JobNameReturns("some-job")
						dbBuild.PipelineNameReturns("a-pipeline")
						dbBuild.TeamNameReturns("some-team")
						dbBuild.StatusReturns(db.BuildStatusSucceeded)
						dbBuild.StartTimeReturns(time.Unix(1, 0))
						dbBuild.EndTimeReturns(time.Unix(100, 0))
						fakeJob.BuildReturns(dbBuild, true, nil)
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

					It("fetches by job and build name", func() {
						Expect(fakeJob.BuildCallCount()).To(Equal(1))

						buildName := fakeJob.BuildArgsForCall(0)
						Expect(buildName).To(Equal("some-build"))
					})

					It("returns the build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
					"id": 1,
					"name": "1",
					"job_name": "some-job",
					"status": "succeeded",
					"api_url": "/api/v1/builds/1",
					"pipeline_name": "a-pipeline",
					"team_name": "some-team",
					"start_time": 1,
					"end_time": 100
				}`))

					})
				})

				Context("when the build is not found", func() {
					BeforeEach(func() {
						fakeJob.BuildReturns(nil, false, nil)
					})

					It("returns Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when getting the build fails", func() {
					BeforeEach(func() {
						fakeJob.BuildReturns(nil, false, errors.New("oh no!"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, errors.New("oh no!"))
				})

				It("returns Internal Server Error", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(false)
					})
					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						fakeAccess.IsAuthenticatedReturns(true)
					})

					It("returns 403", func() {
						Expect(response.StatusCode).To(Equal(http.StatusForbidden))
					})
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					fakeBuild := new(dbfakes.FakeBuild)
					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.BuildReturns(fakeBuild, true, nil)

					fakePipeline.PublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})
	})

	Describe("POST /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/some-build", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized and authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
				fakeAccess.IsAuthenticatedReturns(true)
			})

			Context("when getting the job fails", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, errors.New("errorrr"))
				})

				It("returns a 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns a 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when getting the job succeeds", func() {
				BeforeEach(func() {
					fakeJob.NameReturns("some-job")
					fakePipeline.JobReturns(fakeJob, true, nil)
				})

				It("tries to get the build to rerun", func() {
					Expect(fakeJob.BuildCallCount()).To(Equal(1))
				})

				Context("when getting the build to rerun fails", func() {
					BeforeEach(func() {
						fakeJob.BuildReturns(nil, false, errors.New("oops"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the build to rerun is not found", func() {
					BeforeEach(func() {
						fakeJob.BuildReturns(nil, false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when getting the build to rerun succeeds", func() {
					var fakeBuild *dbfakes.FakeBuild
					BeforeEach(func() {
						fakeBuild = new(dbfakes.FakeBuild)
						fakeBuild.IDReturns(1)
						fakeBuild.NameReturns("1")
						fakeBuild.JobNameReturns("some-job")
						fakeBuild.PipelineNameReturns("a-pipeline")
						fakeBuild.TeamNameReturns("some-team")
						fakeBuild.StatusReturns(db.BuildStatusStarted)
						fakeBuild.StartTimeReturns(time.Unix(1, 0))
						fakeBuild.EndTimeReturns(time.Unix(100, 0))

						fakeJob.BuildReturns(fakeBuild, true, nil)
					})

					Context("when the build has no inputs", func() {
						BeforeEach(func() {
							fakeBuild.InputsReadyReturns(false)
						})

						It("returns a 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})

					Context("when the build is input ready", func() {
						BeforeEach(func() {
							fakeBuild.InputsReadyReturns(true)
						})
						Context("when creating the rerun build fails", func() {
							BeforeEach(func() {
								fakeJob.RerunBuildReturns(nil, errors.New("nopers"))
							})

							It("returns a 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})

						Context("when creating the rerun build succeeds", func() {
							BeforeEach(func() {
								build := new(dbfakes.FakeBuild)
								build.IDReturns(2)
								build.NameReturns("1.1")
								build.JobNameReturns("some-job")
								build.PipelineNameReturns("a-pipeline")
								build.TeamNameReturns("some-team")
								build.StatusReturns(db.BuildStatusStarted)
								build.StartTimeReturns(time.Unix(1, 0))
								build.EndTimeReturns(time.Unix(100, 0))

								fakeJob.RerunBuildReturns(build, nil)
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

							It("returns the build", func() {
								body, err := ioutil.ReadAll(response.Body)
								Expect(err).NotTo(HaveOccurred())

								Expect(body).To(MatchJSON(`{
							"id": 2,
							"name": "1.1",
							"job_name": "some-job",
							"status": "started",
							"api_url": "/api/v1/builds/2",
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"start_time": 1,
							"end_time": 100
						}`))
							})
						})
					})
				})
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/pause", nil)
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

					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.PauseReturns(nil)
				})

				It("finds the job on the pipeline and pauses it", func() {
					jobName := fakePipeline.JobArgsForCall(0)
					Expect(jobName).To(Equal("job-name"))

					Expect(fakeJob.PauseCallCount()).To(Equal(1))

					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job fails to be paused", func() {
					BeforeEach(func() {
						fakeJob.PauseReturns(errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Status Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)

					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.UnpauseReturns(nil)
				})

				It("finds the job on the pipeline and unpauses it", func() {
					jobName := fakePipeline.JobArgsForCall(0)
					Expect(jobName).To(Equal("job-name"))

					Expect(fakeJob.UnpauseCallCount()).To(Equal(1))

					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job fails to be unpaused", func() {
					BeforeEach(func() {
						fakeJob.UnpauseReturns(errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Status Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("DELETE /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/tasks/:step_name/cache", func() {
		var (
			request  *http.Request
			response *http.Response
		)

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("DELETE", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/tasks/:step_name/cache", nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthorizedReturns(true)
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeAccess.IsAuthenticatedReturns(true)

					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.ClearTaskCacheReturns(1, nil)

				})

				Context("when no cachePath is passed", func() {
					It("it finds the right job", func() {
						jobName := fakePipeline.JobArgsForCall(0)
						Expect(jobName).To(Equal("job-name"))
					})

					It("it clears the db cache entries successfully", func() {
						Expect(fakeJob.ClearTaskCacheCallCount()).To(Equal(1))
						_, cachePath := fakeJob.ClearTaskCacheArgsForCall(0)
						Expect(cachePath).To(Equal(""))
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

					It("it returns the number of rows deleted", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"caches_removed": 1}`))
					})

					Context("but no rows were deleted", func() {
						BeforeEach(func() {
							fakeJob.ClearTaskCacheReturns(0, nil)
						})

						It("it returns that 0 rows were deleted", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{"caches_removed": 0}`))
						})

					})
				})

				Context("when a cachePath is passed", func() {
					BeforeEach(func() {
						query := request.URL.Query()
						query.Add(atc.ClearTaskCacheQueryPath, "cache-path")
						request.URL.RawQuery = query.Encode()
					})

					It("it finds the right job", func() {
						jobName := fakePipeline.JobArgsForCall(0)
						Expect(jobName).To(Equal("job-name"))
					})

					It("it clears the db cache entries successfully", func() {
						Expect(fakeJob.ClearTaskCacheCallCount()).To(Equal(1))
						_, cachePath := fakeJob.ClearTaskCacheArgsForCall(0)
						Expect(cachePath).To(Equal("cache-path"))
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

					It("it returns the number of rows deleted", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{"caches_removed": 1}`))
					})

					Context("but no rows corresponding to the cachePath are deleted", func() {
						BeforeEach(func() {
							fakeJob.ClearTaskCacheReturns(0, nil)
						})

						It("it returns that 0 rows were deleted", func() {
							body, err := ioutil.ReadAll(response.Body)
							Expect(err).NotTo(HaveOccurred())

							Expect(body).To(MatchJSON(`{"caches_removed": 0}`))
						})
					})
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when there are problems removing the db cache entries", func() {
					BeforeEach(func() {
						fakeJob.ClearTaskCacheReturns(-1, errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Status Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name/schedule", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/job-name/schedule", nil)
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

					fakePipeline.JobReturns(fakeJob, true, nil)
					fakeJob.RequestScheduleReturns(nil)
				})

				It("finds the job on the pipeline and schedules it", func() {
					jobName := fakePipeline.JobArgsForCall(0)
					Expect(jobName).To(Equal("job-name"))

					Expect(fakeJob.RequestScheduleCallCount()).To(Equal(1))

					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when the job is not found", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, nil)
					})

					It("returns a 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when finding the job fails", func() {
					BeforeEach(func() {
						fakePipeline.JobReturns(nil, false, errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job fails to be scheduled", func() {
					BeforeEach(func() {
						fakeJob.RequestScheduleReturns(errors.New("some-error"))
					})

					It("returns a 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeAccess.IsAuthenticatedReturns(false)
			})

			It("returns Status Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})

func fakeDBResourceType(t atc.VersionedResourceType) *dbfakes.FakeResourceType {
	fake := new(dbfakes.FakeResourceType)
	fake.NameReturns(t.Name)
	fake.TypeReturns(t.Type)
	fake.SourceReturns(t.Source)
	fake.VersionReturns(t.Version)
	return fake
}
