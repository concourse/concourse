package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/scheduler/schedulerfakes"
)

var _ = Describe("Jobs API", func() {
	var pipelineDB *dbfakes.FakePipelineDB
	var expectedSavedPipeline db.SavedPipeline
	var versionedResourceTypes atc.VersionedResourceTypes

	BeforeEach(func() {
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildReturns(pipelineDB)
		expectedSavedPipeline = db.SavedPipeline{}
		teamDB.GetPipelineByNameReturns(expectedSavedPipeline, true, nil)

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

		fakePipeline.ResourceTypesReturns([]dbng.ResourceType{
			fakeDBNGResourceType(versionedResourceTypes[0]),
			fakeDBNGResourceType(versionedResourceTypes[1]),
			fakeDBNGResourceType(versionedResourceTypes[2]),
		}, nil)
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs/:job_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
				userContextReader.GetTeamReturns("", false, false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					pipelineDB.IsPublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					pipelineDB.GetJobReturns(db.SavedJob{}, true, nil)
					pipelineDB.IsPublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			Context("when getting the build succeeds", func() {
				BeforeEach(func() {
					build1 := new(dbfakes.FakeBuild)
					build1.IDReturns(1)
					build1.NameReturns("1")
					build1.JobNameReturns("some-job")
					build1.PipelineNameReturns("some-pipeline")
					build1.TeamNameReturns("some-team")
					build1.StatusReturns(db.StatusSucceeded)
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(3)
					build2.NameReturns("2")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.TeamNameReturns("some-team")
					build2.StatusReturns(db.StatusStarted)

					pipelineDB.GetJobFinishedAndNextBuildReturns(build1, build2, nil)
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{}, false, errors.New("nope"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{
							ID:                 1,
							Paused:             true,
							FirstLoggedBuildID: 99,
							PipelineName:       "some-pipeline",
							Job: db.Job{
								Name: "some-job",
							},
							Config: atc.JobConfig{
								Name: "some-job",
								Plan: atc.PlanSequence{
									{
										Get: "some-input",
									},
									{
										Get:      "some-name",
										Resource: "some-other-input",
										Params:   atc.Params{"secret": "params"},
										Passed:   []string{"a", "b"},
										Trigger:  true,
									},
									{
										Put: "some-output",
									},
									{
										Put:    "some-other-output",
										Params: atc.Params{"secret": "params"},
									},
								},
							},
						}, true, nil)
						pipelineDB.ConfigReturns(atc.Config{
							Groups: []atc.GroupConfig{
								{
									Name: "group-1",
									Jobs: []string{"some-job"},
								},
								{
									Name: "group-2",
									Jobs: []string{"some-job"},
								},
							},
						})
					})

					It("fetches by job", func() {
						Expect(pipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))

						jobName := pipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)
						Expect(jobName).To(Equal("some-job"))
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the job's name, url, if it's paused, and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"id": 1,
							"name": "some-job",
							"paused": true,
							"first_logged_build_id": 99,
							"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job",
							"next_build": {
								"id": 3,
								"name": "2",
								"job_name": "some-job",
								"status": "started",
								"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/2",
								"api_url": "/api/v1/builds/3",
								"pipeline_name": "some-pipeline",
								"team_name": "some-team"
							},
							"finished_build": {
								"id": 1,
								"name": "1",
								"job_name": "some-job",
								"status": "succeeded",
								"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/1",
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
							pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
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
							pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})

			Context("when the job is not found", func() {
				BeforeEach(func() {
					pipelineDB.GetJobReturns(db.SavedJob{}, false, nil)
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

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.ConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name: "group-1",
							Jobs: []string{"some-job"},
						},
						{
							Name: "group-2",
							Jobs: []string{"some-job"},
						},
					},

					Jobs: []atc.JobConfig{
						{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
								{
									Get:      "some-name",
									Resource: "some-other-input",
									Params:   atc.Params{"secret": "params"},
									Passed:   []string{"a", "b"},
									Trigger:  true,
								},
								{
									Put: "some-output",
								},
								{
									Put:    "some-other-output",
									Params: atc.Params{"secret": "params"},
								},
							},
						},
					},
				})
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(false)
					userContextReader.GetTeamReturns("", false, false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(true)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(true)
					userContextReader.GetTeamReturns("some-team", true, true)
				})

				It("fetches by job", func() {
					Expect(pipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))

					jobName := pipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)
					Expect(jobName).To(Equal("some-job"))
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
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
						build1.StatusReturns(db.StatusSucceeded)

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(3)
						build2.NameReturns("2")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("some-pipeline")
						build2.StatusReturns(db.StatusStarted)

						pipelineDB.GetJobFinishedAndNextBuildReturns(build1, build2, nil)
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
						build1.StatusReturns(db.StatusFailed)

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(3)
						build2.NameReturns("2")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("some-pipeline")
						build2.StatusReturns(db.StatusStarted)

						pipelineDB.GetJobFinishedAndNextBuildReturns(build1, build2, nil)
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
						build1.StatusReturns(db.StatusAborted)

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(1)
						build2.NameReturns("1")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("some-pipeline")
						build2.StartTimeReturns(time.Unix(1, 0))
						build2.EndTimeReturns(time.Unix(100, 0))
						build2.StatusReturns(db.StatusAborted)

						pipelineDB.GetJobFinishedAndNextBuildReturns(build1, build2, nil)
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
						build1.StatusReturns(db.StatusErrored)

						build2 := new(dbfakes.FakeBuild)
						build2.IDReturns(3)
						build2.NameReturns("2")
						build2.JobNameReturns("some-job")
						build2.PipelineNameReturns("some-pipeline")
						build2.StatusReturns(db.StatusStarted)

						pipelineDB.GetJobFinishedAndNextBuildReturns(build1, build2, nil)
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
						pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
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
						pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job is not present in the config", func() {
					BeforeEach(func() {
						pipelineDB.ConfigReturns(atc.Config{
							Jobs: []atc.JobConfig{
								{Name: "other-job"},
							},
						})
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})
			})
		})
	})

	Describe("GET /api/v1/teams/:team_name/pipelines/:pipeline_name/jobs", func() {
		var response *http.Response
		var dashboardResponse db.Dashboard
		var groups []atc.GroupConfig

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the dashboard succeeds", func() {
			BeforeEach(func() {
				groups = []atc.GroupConfig{
					{
						Name: "group-1",
						Jobs: []string{"job-1"},
					},
					{
						Name: "group-2",
						Jobs: []string{"job-1", "job-2"},
					},
				}

				job1 := db.SavedJob{
					ID:           1,
					Paused:       true,
					PipelineName: "another-pipeline",
					Job: db.Job{
						Name: "job-1",
					},
					Config: atc.JobConfig{
						Name: "job-1",
						Plan: atc.PlanSequence{{Get: "input-1"}, {Put: "output-1"}},
					},
				}

				job2 := db.SavedJob{
					ID:           2,
					Paused:       true,
					PipelineName: "another-pipeline",
					Job: db.Job{
						Name: "job-2",
					},
					Config: atc.JobConfig{
						Name: "job-2",
						Plan: atc.PlanSequence{{Get: "input-2"}, {Put: "output-2"}},
					},
				}

				job3 := db.SavedJob{
					ID:           3,
					Paused:       true,
					PipelineName: "another-pipeline",
					Job: db.Job{
						Name: "job-3",
					},
					Config: atc.JobConfig{
						Name: "job-3",
						Plan: atc.PlanSequence{{Get: "input-3"}, {Put: "output-3"}},
					},
				}

				nextBuild1 := new(dbfakes.FakeBuild)
				nextBuild1.IDReturns(3)
				nextBuild1.NameReturns("2")
				nextBuild1.JobNameReturns("job-1")
				nextBuild1.PipelineNameReturns("another-pipeline")
				nextBuild1.TeamNameReturns("some-team")
				nextBuild1.StatusReturns(db.StatusStarted)

				finishedBuild1 := new(dbfakes.FakeBuild)
				finishedBuild1.IDReturns(1)
				finishedBuild1.NameReturns("1")
				finishedBuild1.JobNameReturns("job-1")
				finishedBuild1.PipelineNameReturns("another-pipeline")
				finishedBuild1.TeamNameReturns("some-team")
				finishedBuild1.StatusReturns(db.StatusSucceeded)
				finishedBuild1.StartTimeReturns(time.Unix(1, 0))
				finishedBuild1.EndTimeReturns(time.Unix(100, 0))

				finishedBuild2 := new(dbfakes.FakeBuild)
				finishedBuild2.IDReturns(4)
				finishedBuild2.NameReturns("1")
				finishedBuild2.JobNameReturns("job-2")
				finishedBuild2.PipelineNameReturns("another-pipeline")
				finishedBuild2.TeamNameReturns("some-team")
				finishedBuild2.StatusReturns(db.StatusSucceeded)
				finishedBuild2.StartTimeReturns(time.Unix(101, 0))
				finishedBuild2.EndTimeReturns(time.Unix(200, 0))

				dashboardResponse = db.Dashboard{
					{
						Job:           job1,
						NextBuild:     nextBuild1,
						FinishedBuild: finishedBuild1,
					},
					{
						Job:           job2,
						NextBuild:     nil,
						FinishedBuild: finishedBuild2,
					},
					{
						Job:           job3,
						NextBuild:     nil,
						FinishedBuild: nil,
					},
				}
				pipelineDB.GetDashboardReturns(dashboardResponse, groups, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(false)
					userContextReader.GetTeamReturns("", false, false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(true)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(true)
					userContextReader.GetTeamReturns("some-team", true, true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns each job's name, url, and any running and finished builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Expect(err).NotTo(HaveOccurred())

					Expect(body).To(MatchJSON(`[
							{
								"id": 1,
								"name": "job-1",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1",
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/2",
									"api_url": "/api/v1/builds/3",
									"pipeline_name": "another-pipeline",
									"team_name": "some-team"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/1",
									"api_url": "/api/v1/builds/1",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team",
									"start_time": 1,
									"end_time": 100
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": false}],
								"outputs": [{"name": "output-1", "resource": "output-1"}],
								"groups": ["group-1", "group-2"]
							},
							{
								"id": 2,
								"name": "job-2",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2",
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2/builds/1",
									"api_url": "/api/v1/builds/4",
									"pipeline_name": "another-pipeline",
									"team_name": "some-team",
									"start_time": 101,
									"end_time": 200
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": false}],
								"outputs": [{"name": "output-2", "resource": "output-2"}],
								"groups": ["group-2"]
							},
							{
								"id": 3,
								"name": "job-3",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-3",
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": false}],
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
				})

				Context("when manual triggering of a job is disabled", func() {
					BeforeEach(func() {
						dashboardResponse[0].Job.Config.DisableManualTrigger = true
						pipelineDB.GetDashboardReturns(dashboardResponse, groups, nil)
					})

					It("returns each job's name, url, manual trigger state and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"id": 1,
								"name": "job-1",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1",
								"disable_manual_trigger": true,
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/2",
									"api_url": "/api/v1/builds/3",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-1/builds/1",
									"api_url": "/api/v1/builds/1",
									"pipeline_name":"another-pipeline",
									"team_name": "some-team",
									"start_time": 1,
									"end_time": 100
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": false}],
								"outputs": [{"name": "output-1", "resource": "output-1"}],
								"groups": ["group-1", "group-2"]
							},
							{
								"id": 2,
								"name": "job-2",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2",
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-2/builds/1",
									"api_url": "/api/v1/builds/4",
									"pipeline_name": "another-pipeline",
									"team_name": "some-team",
									"start_time": 101,
									"end_time": 200
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": false}],
								"outputs": [{"name": "output-2", "resource": "output-2"}],
								"groups": ["group-2"]
							},
							{
								"id": 3,
								"name": "job-3",
								"paused": true,
								"url": "/teams/some-team/pipelines/another-pipeline/jobs/job-3",
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": false}],
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
					})
				})

				Context("when getting the dashboard fails", func() {
					Context("with an unknown error", func() {
						BeforeEach(func() {
							pipelineDB.GetDashboardReturns(nil, nil, errors.New("oh no!"))
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

			pipelineDB.GetPipelineNameReturns("some-pipeline")
			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds" + queryParams)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
				userContextReader.GetTeamReturns("", false, false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					pipelineDB.IsPublicReturns(false)
				})

				It("returns 401", func() {
					Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})

			Context("and the pipeline is public", func() {
				BeforeEach(func() {
					pipelineDB.IsPublicReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			Context("when no params are passed", func() {
				It("does not set defaults for since and until", func() {
					Expect(pipelineDB.GetJobBuildsCallCount()).To(Equal(1))

					jobName, page := pipelineDB.GetJobBuildsArgsForCall(0)
					Expect(jobName).To(Equal("some-job"))
					Expect(page).To(Equal(db.Page{
						Since: 0,
						Until: 0,
						Limit: 100,
					}))
				})
			})

			Context("when all the params are passed", func() {
				BeforeEach(func() {
					queryParams = "?since=2&until=3&limit=8"
				})

				It("passes them through", func() {
					Expect(pipelineDB.GetJobBuildsCallCount()).To(Equal(1))

					jobName, page := pipelineDB.GetJobBuildsArgsForCall(0)
					Expect(jobName).To(Equal("some-job"))
					Expect(page).To(Equal(db.Page{
						Since: 2,
						Until: 3,
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
					build1.StatusReturns(db.StatusStarted)
					build1.StartTimeReturns(time.Unix(1, 0))
					build1.EndTimeReturns(time.Unix(100, 0))

					build2 := new(dbfakes.FakeBuild)
					build2.IDReturns(2)
					build2.NameReturns("1")
					build2.JobNameReturns("some-job")
					build2.PipelineNameReturns("some-pipeline")
					build2.TeamNameReturns("some-team")
					build2.StatusReturns(db.StatusSucceeded)
					build2.StartTimeReturns(time.Unix(101, 0))
					build2.EndTimeReturns(time.Unix(200, 0))

					returnedBuilds = []db.Build{build1, build2}
					pipelineDB.GetJobBuildsReturns(returnedBuilds, db.Pagination{}, nil)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
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
						"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/2",
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
						"url": "/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds/1",
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
						pipelineDB.GetJobBuildsReturns(returnedBuilds, db.Pagination{
							Previous: &db.Page{Until: 4, Limit: 2},
							Next:     &db.Page{Since: 2, Limit: 2},
						}, nil)
					})

					It("returns Link headers per rfc5988", func() {
						Expect(response.Header["Link"]).To(ConsistOf([]string{
							fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?until=4&limit=2>; rel="previous"`, externalURL),
							fmt.Sprintf(`<%s/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds?since=2&limit=2>; rel="next"`, externalURL),
						}))
					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					pipelineDB.GetJobBuildsReturns(nil, db.Pagination{}, errors.New("oh no!"))
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

		var fakeScheduler *schedulerfakes.FakeBuildScheduler

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("POST", server.URL+"/api/v1/teams/some-team/pipelines/some-pipeline/jobs/some-job/builds", nil)
			Expect(err).NotTo(HaveOccurred())

			fakeScheduler = new(schedulerfakes.FakeBuildScheduler)
			fakeSchedulerFactory.BuildSchedulerReturns(fakeScheduler)
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			Context("when manual triggering is disabled", func() {
				BeforeEach(func() {
					pipelineDB.ConfigReturns(atc.Config{
						Jobs: []atc.JobConfig{
							{
								Name:                 "some-job",
								DisableManualTrigger: true,
								Plan: atc.PlanSequence{
									{
										Get: "some-input",
									},
								},
							},
						},

						Resources: atc.ResourceConfigs{
							{Name: "resource-1", Type: "some-type"},
							{Name: "resource-2", Type: "some-other-type"},
						},
					})
				})

				It("should return 409", func() {
					Expect(response.StatusCode).To(Equal(http.StatusConflict))
				})

				It("does not trigger the build", func() {
					Expect(fakeScheduler.TriggerImmediatelyCallCount()).To(Equal(0))
				})
			})

			Context("when getting the job config succeeds", func() {
				BeforeEach(func() {
					pipelineDB.ConfigReturns(atc.Config{
						Jobs: []atc.JobConfig{
							{
								Name: "some-job",
								Plan: atc.PlanSequence{
									{
										Get: "some-input",
									},
								},
							},
						},

						Resources: atc.ResourceConfigs{
							{Name: "resource-1", Type: "some-type"},
							{Name: "resource-2", Type: "some-other-type"},
						},
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
						build.StatusReturns(db.StatusStarted)
						build.StartTimeReturns(time.Unix(1, 0))
						build.EndTimeReturns(time.Unix(100, 0))
						fakeScheduler.TriggerImmediatelyReturns(build, nil, nil)
					})

					It("triggers using the current config", func() {
						Expect(fakeScheduler.TriggerImmediatelyCallCount()).To(Equal(1))

						_, job, resources, resourceTypes := fakeScheduler.TriggerImmediatelyArgsForCall(0)
						Expect(job).To(Equal(atc.JobConfig{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
							},
						}))
						Expect(resources).To(Equal(atc.ResourceConfigs{
							{Name: "resource-1", Type: "some-type"},
							{Name: "resource-2", Type: "some-other-type"},
						}))
						Expect(resourceTypes).To(Equal(versionedResourceTypes))
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns the build", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`{
							"id": 42,
							"name": "1",
							"job_name": "some-job",
							"status": "started",
							"url": "/teams/some-team/pipelines/a-pipeline/jobs/some-job/builds/1",
							"api_url": "/api/v1/builds/42",
							"pipeline_name": "a-pipeline",
							"team_name": "some-team",
							"start_time": 1,
							"end_time": 100
						}`))
					})
				})

				Context("when triggering the build fails", func() {
					BeforeEach(func() {
						fakeScheduler.TriggerImmediatelyReturns(nil, nil, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
					})
				})

				Context("when the job is not present in the config", func() {
					BeforeEach(func() {
						pipelineDB.ConfigReturns(atc.Config{
							Jobs: []atc.JobConfig{
								{Name: "other-job"},
							},
						})
					})

					It("returns 404", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
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

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			It("looked up the proper pipeline", func() {
				Expect(teamDB.GetPipelineByNameCallCount()).To(Equal(1))
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
			})

			Context("when getting the config succeeds", func() {
				Context("when it contains the requested job", func() {
					someJob := atc.JobConfig{
						Name: "some-job",
						Plan: atc.PlanSequence{
							{
								Get:      "some-input",
								Resource: "some-resource",
								Passed:   []string{"job-a", "job-b"},
								Params:   atc.Params{"some": "params"},
							},
							{
								Get:      "some-other-input",
								Resource: "some-other-resource",
								Passed:   []string{"job-c", "job-d"},
								Params:   atc.Params{"some": "other-params"},
								Tags:     []string{"some-tag"},
							},
						},
					}

					var fakeScheduler *schedulerfakes.FakeBuildScheduler
					BeforeEach(func() {
						fakeScheduler = new(schedulerfakes.FakeBuildScheduler)
						fakeSchedulerFactory.BuildSchedulerReturns(fakeScheduler)
						pipelineDB.ConfigReturns(atc.Config{
							Jobs: atc.JobConfigs{
								someJob,
							},

							Resources: atc.ResourceConfigs{
								{
									Name:   "some-resource",
									Source: atc.Source{"some": "source"},
								},
								{
									Name:   "some-other-resource",
									Source: atc.Source{"some": "other-source"},
								},
							},
						})
					})

					Context("when the input versions for the job can be determined", func() {
						BeforeEach(func() {
							pipelineDB.GetNextBuildInputsStub = func(string) ([]db.BuildInput, bool, error) {
								defer GinkgoRecover()
								Expect(fakeScheduler.SaveNextInputMappingCallCount()).To(Equal(1))
								return []db.BuildInput{
									{
										Name: "some-input",
										VersionedResource: db.VersionedResource{
											Resource:   "some-resource",
											Type:       "some-type",
											Version:    db.Version{"some": "version"},
											PipelineID: 42,
										},
									},
									{
										Name: "some-other-input",
										VersionedResource: db.VersionedResource{
											Resource:   "some-other-resource",
											Type:       "some-other-type",
											Version:    db.Version{"some": "other-version"},
											PipelineID: 42,
										},
									},
								}, true, nil
							}
						})

						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("created the scheduler with the correct pipelineDB and external URL", func() {
							actualPipelineDB, _, actualExternalURL := fakeSchedulerFactory.BuildSchedulerArgsForCall(0)
							Expect(actualPipelineDB).To(Equal(pipelineDB))
							Expect(actualExternalURL).To(Equal(externalURL))
						})

						It("determined the inputs with the correct job config", func() {
							_, receivedJobConfig := fakeScheduler.SaveNextInputMappingArgsForCall(0)
							Expect(receivedJobConfig).To(Equal(someJob))
						})

						It("loaded the inputs with the correct job name", func() {
							receivedJobName := pipelineDB.GetNextBuildInputsArgsForCall(0)
							Expect(receivedJobName).To(Equal("some-job"))
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

					Context("when the job has no input versions available", func() {
						BeforeEach(func() {
							pipelineDB.GetNextBuildInputsReturns(nil, false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when the input versions for the job can not be determined", func() {
						BeforeEach(func() {
							pipelineDB.GetNextBuildInputsReturns(nil, false, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})

			Context("when it does not contain the requested job", func() {
				BeforeEach(func() {
					pipelineDB.ConfigReturns(atc.Config{
						Jobs: atc.JobConfigs{
							{
								Name: "some-bogus-job",
								Plan: atc.PlanSequence{},
							},
						},
					})
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
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

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				build.IDReturns(1)
				build.NameReturns("1")
				build.JobNameReturns("some-job")
				build.PipelineNameReturns("a-pipeline")
				build.TeamNameReturns("some-team")
				build.StatusReturns(db.StatusSucceeded)
				build.StartTimeReturns(time.Unix(1, 0))
				build.EndTimeReturns(time.Unix(100, 0))
				pipelineDB.GetJobBuildReturns(build, true, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(false)
					userContextReader.GetTeamReturns("", false, false)
				})

				Context("and the pipeline is private", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(false)
					})

					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("and the pipeline is public", func() {
					BeforeEach(func() {
						pipelineDB.IsPublicReturns(true)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})
				})
			})

			Context("when authorized", func() {
				BeforeEach(func() {
					authValidator.IsAuthenticatedReturns(true)
					userContextReader.GetTeamReturns("some-team", true, true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("fetches by job and build name", func() {
					Expect(pipelineDB.GetJobBuildCallCount()).To(Equal(1))

					jobName, buildName := pipelineDB.GetJobBuildArgsForCall(0)
					Expect(jobName).To(Equal("some-job"))
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
					"url": "/teams/some-team/pipelines/a-pipeline/jobs/some-job/builds/1",
					"api_url": "/api/v1/builds/1",
					"pipeline_name": "a-pipeline",
					"team_name": "some-team",
					"start_time": 1,
					"end_time": 100
				}`))

				})

				Context("when the build is not found", func() {
					BeforeEach(func() {
						pipelineDB.GetJobBuildReturns(nil, false, nil)
					})

					It("returns Not Found", func() {
						Expect(response.StatusCode).To(Equal(http.StatusNotFound))
					})
				})

				Context("when getting the build fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobBuildReturns(nil, false, errors.New("oh no!"))
					})

					It("returns Internal Server Error", func() {
						Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
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

		Context("when authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			It("injects the PipelineDB", func() {
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Expect(pipelineDB.PauseJobArgsForCall(0)).To(Equal("job-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
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
				authValidator.IsAuthenticatedReturns(true)
				userContextReader.GetTeamReturns("some-team", true, true)
			})

			It("injects the PipelineDB", func() {
				pipelineName := teamDB.GetPipelineByNameArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
				Expect(pipelineDBFactory.BuildCallCount()).To(Equal(1))
				actualSavedPipeline := pipelineDBFactory.BuildArgsForCall(0)
				Expect(actualSavedPipeline).To(Equal(expectedSavedPipeline))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Expect(pipelineDB.UnpauseJobArgsForCall(0)).To(Equal("job-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authorized", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})

func fakeDBNGResourceType(t atc.VersionedResourceType) *dbngfakes.FakeResourceType {
	fake := new(dbngfakes.FakeResourceType)
	fake.NameReturns(t.Name)
	fake.TypeReturns(t.Type)
	fake.SourceReturns(t.Source)
	fake.VersionReturns(t.Version)
	return fake
}
