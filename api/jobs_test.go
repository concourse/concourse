package api_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/accessor/accessorfakes"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/scheduler/schedulerfakes"
)

var _ = Describe("Jobs API", func() {
	var fakeJob *dbfakes.FakeJob
	var fakeaccess *accessorfakes.FakeAccess
	var versionedResourceTypes atc.VersionedResourceTypes
	var fakePipeline *dbfakes.FakePipeline
	var variables creds.Variables

	BeforeEach(func() {
		fakeJob = new(dbfakes.FakeJob)
		fakeaccess = new(accessorfakes.FakeAccess)
		fakePipeline = new(dbfakes.FakePipeline)
		dbTeamFactory.FindTeamReturns(dbTeam, true, nil)
		dbTeam.PipelineReturns(fakePipeline, true, nil)

		variables = template.StaticVariables{
			"some-param": "lol",
		}
		fakeVariablesFactory.NewVariablesReturns(variables)

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

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
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
			build1 := new(dbfakes.FakeBuild)
			build1.IDReturns(1)
			build1.NameReturns("1")
			build1.JobNameReturns("some-job")
			build1.PipelineNameReturns("some-pipeline")
			build1.TeamNameReturns("some-team")
			build1.StatusReturns(db.BuildStatusSucceeded)
			build1.StartTimeReturns(time.Unix(1, 0))
			build1.EndTimeReturns(time.Unix(100, 0))

			build2 := new(dbfakes.FakeBuild)
			build2.IDReturns(3)
			build2.NameReturns("2")
			build2.JobNameReturns("some-job")
			build2.PipelineNameReturns("some-pipeline")
			build2.TeamNameReturns("some-team")
			build2.StatusReturns(db.BuildStatusStarted)

			fakeJob.IDReturns(1)
			fakeJob.PausedReturns(true)
			fakeJob.FirstLoggedBuildIDReturns(99)
			fakeJob.PipelineNameReturns("some-pipeline")
			fakeJob.NameReturns("some-job")
			fakeJob.ConfigReturns(atc.JobConfig{
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
			})
			fakeJob.TagsReturns([]string{"group-1", "group-2"})

			fakeJob.TeamNameReturns("some-team")

			fakePipeline.JobReturns(fakeJob, true, nil)

			dbJobFactory.VisibleJobsReturns(db.Dashboard{
				db.DashboardJob{
					Job:           fakeJob,
					NextBuild:     build2,
					FinishedBuild: build1,
				},
			}, nil)
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				"first_logged_build_id": 99,
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

		Context("when not authenticated", func() {
			It("populates job factory with no team names", func() {
				Expect(dbJobFactory.VisibleJobsCallCount()).To(Equal(1))
				Expect(dbJobFactory.VisibleJobsArgsForCall(0)).To(BeEmpty())
			})
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				fakeaccess.TeamNamesReturns([]string{"some-team"})
			})

			It("constructs job factory with provided team names", func() {
				Expect(dbJobFactory.VisibleJobsCallCount()).To(Equal(1))
				Expect(dbJobFactory.VisibleJobsArgsForCall(0)).To(ContainElement("some-team"))
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
				fakeaccess.IsAuthenticatedReturns(false)
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
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
						fakeJob.ConfigReturns(atc.JobConfig{
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
						})
						fakeJob.TagsReturns([]string{"group-1", "group-2"})
						fakeJob.FinishedAndNextBuildReturns(build1, build2, nil)

						fakePipeline.JobReturns(fakeJob, true, nil)
					})

					It("fetches by job", func() {
						Expect(fakeJob.FinishedAndNextBuildCallCount()).To(Equal(1))
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(false)
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
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)

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
				Expect(response.Header.Get("Content-Type")).To(Equal("image/svg+xml"))
				Expect(response.Header.Get("Cache-Control")).To(Equal("no-cache, no-store, must-revalidate"))
				Expect(response.Header.Get("Expires")).To(Equal("0"))
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
		var dashboardResponse db.Dashboard

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/teams/some-team/pipelines/some-pipeline/jobs")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when getting the dashboard succeeds", func() {
			var job1 *dbfakes.FakeJob

			BeforeEach(func() {
				job1 = new(dbfakes.FakeJob)
				job1.IDReturns(1)
				job1.PausedReturns(true)
				job1.PipelineNameReturns("another-pipeline")
				job1.NameReturns("job-1")
				job1.ConfigReturns(atc.JobConfig{
					Name: "job-1",
					Plan: atc.PlanSequence{{Get: "input-1"}, {Put: "output-1"}},
				})
				job1.TagsReturns([]string{"group-1", "group-2"})

				job2 := new(dbfakes.FakeJob)
				job2.IDReturns(2)
				job2.PausedReturns(true)
				job2.PipelineNameReturns("another-pipeline")
				job2.NameReturns("job-2")
				job2.ConfigReturns(atc.JobConfig{
					Name: "job-2",
					Plan: atc.PlanSequence{{Get: "input-2"}, {Put: "output-2"}},
				})
				job2.TagsReturns([]string{"group-2"})

				job3 := new(dbfakes.FakeJob)
				job3.IDReturns(3)
				job3.PausedReturns(true)
				job3.PipelineNameReturns("another-pipeline")
				job3.NameReturns("job-3")
				job3.ConfigReturns(atc.JobConfig{
					Name: "job-3",
					Plan: atc.PlanSequence{{Get: "input-3"}, {Put: "output-3"}},
				})
				job3.TagsReturns([]string{})

				nextBuild1 := new(dbfakes.FakeBuild)
				nextBuild1.IDReturns(3)
				nextBuild1.NameReturns("2")
				nextBuild1.JobNameReturns("job-1")
				nextBuild1.PipelineNameReturns("another-pipeline")
				nextBuild1.TeamNameReturns("some-team")
				nextBuild1.StatusReturns(db.BuildStatusStarted)

				finishedBuild1 := new(dbfakes.FakeBuild)
				finishedBuild1.IDReturns(1)
				finishedBuild1.NameReturns("1")
				finishedBuild1.JobNameReturns("job-1")
				finishedBuild1.PipelineNameReturns("another-pipeline")
				finishedBuild1.TeamNameReturns("some-team")
				finishedBuild1.StatusReturns(db.BuildStatusSucceeded)
				finishedBuild1.StartTimeReturns(time.Unix(1, 0))
				finishedBuild1.EndTimeReturns(time.Unix(100, 0))

				finishedBuild2 := new(dbfakes.FakeBuild)
				finishedBuild2.IDReturns(4)
				finishedBuild2.NameReturns("1")
				finishedBuild2.JobNameReturns("job-2")
				finishedBuild2.PipelineNameReturns("another-pipeline")
				finishedBuild2.TeamNameReturns("some-team")
				finishedBuild2.StatusReturns(db.BuildStatusSucceeded)
				finishedBuild2.StartTimeReturns(time.Unix(101, 0))
				finishedBuild2.EndTimeReturns(time.Unix(200, 0))

				transitionBuild := new(dbfakes.FakeBuild)
				transitionBuild.IDReturns(5)
				transitionBuild.NameReturns("five")
				transitionBuild.JobNameReturns("job-1")
				transitionBuild.PipelineNameReturns("another-pipeline")
				transitionBuild.TeamNameReturns("some-team")
				transitionBuild.StatusReturns(db.BuildStatusFailed)
				transitionBuild.StartTimeReturns(time.Unix(101, 0))
				transitionBuild.EndTimeReturns(time.Unix(200, 0))

				dashboardResponse = db.Dashboard{
					{
						Job:             job1,
						NextBuild:       nextBuild1,
						FinishedBuild:   finishedBuild1,
						TransitionBuild: transitionBuild,
					},
					{
						Job:             job2,
						NextBuild:       nil,
						FinishedBuild:   finishedBuild2,
						TransitionBuild: nil,
					},
					{
						Job:             job3,
						NextBuild:       nil,
						FinishedBuild:   nil,
						TransitionBuild: nil,
					},
				}
				fakePipeline.DashboardReturns(dashboardResponse, nil)
			})

			Context("when not authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthorizedReturns(false)
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthenticatedReturns(false)
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
					fakeaccess.IsAuthorizedReturns(true)
					fakeaccess.IsAuthenticatedReturns(true)
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				It("returns Content-Type 'application/json'", func() {
					Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
								"outputs": [{"name": "output-1", "resource": "output-1"}],
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
								"outputs": [{"name": "output-2", "resource": "output-2"}],
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
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
				})

				Context("when manual triggering of a job is disabled", func() {
					BeforeEach(func() {
						job1.ConfigReturns(atc.JobConfig{
							Name:                 "job-1",
							Plan:                 atc.PlanSequence{{Get: "input-1"}, {Put: "output-1"}},
							DisableManualTrigger: true,
						})
						fakePipeline.DashboardReturns(dashboardResponse, nil)
					})

					It("returns each job's name, manual trigger state and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"id": 1,
								"name": "job-1",
								"pipeline_name": "another-pipeline",
								"team_name": "some-team",
								"paused": true,
								"disable_manual_trigger": true,
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"api_url": "/api/v1/builds/3",
									"pipeline_name":"another-pipeline",
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
								"outputs": [{"name": "output-1", "resource": "output-1"}],
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
								"outputs": [{"name": "output-2", "resource": "output-2"}],
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
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
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
				fakeaccess.IsAuthorizedReturns(false)
				fakeaccess.IsAuthenticatedReturns(true)
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
				fakeaccess.IsAuthorizedReturns(true)
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
						Expect(fakeJob.BuildsCallCount()).To(Equal(1))

						page := fakeJob.BuildsArgsForCall(0)
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

						returnedBuilds = []db.Build{build1, build2}
						fakeJob.BuildsReturns(returnedBuilds, db.Pagination{}, nil)
					})

					It("returns 200 OK", func() {
						Expect(response.StatusCode).To(Equal(http.StatusOK))
					})

					It("returns Content-Type 'application/json'", func() {
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
					}
				]`))
					})

					Context("when next/previous pages are available", func() {
						BeforeEach(func() {
							fakeJob.BuildsReturns(returnedBuilds, db.Pagination{
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

		var fakeScheduler *schedulerfakes.FakeBuildScheduler
		var fakeResource *dbfakes.FakeResource
		var fakeResource2 *dbfakes.FakeResource

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

		Context("when authorized and authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(true)
				fakeaccess.IsAuthenticatedReturns(true)
			})

			Context("when getting the job succeeds", func() {
				BeforeEach(func() {
					fakeJob.NameReturns("some-job")
					fakePipeline.JobReturns(fakeJob, true, nil)
				})

				Context("when manual triggering is disabled", func() {
					BeforeEach(func() {
						fakeJob.ConfigReturns(atc.JobConfig{
							Name:                 "some-job",
							DisableManualTrigger: true,
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
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
						fakeJob.ConfigReturns(atc.JobConfig{
							Name: "some-job",
							Plan: atc.PlanSequence{
								{
									Get: "some-input",
								},
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
							build.StatusReturns(db.BuildStatusStarted)
							build.StartTimeReturns(time.Unix(1, 0))
							build.EndTimeReturns(time.Unix(100, 0))
							fakeScheduler.TriggerImmediatelyReturns(build, nil, nil)

							fakeResource = new(dbfakes.FakeResource)
							fakeResource.NameReturns("resource-1")
							fakeResource.TypeReturns("some-type")

							fakeResource2 = new(dbfakes.FakeResource)
							fakeResource2.NameReturns("resource-2")
							fakeResource2.TypeReturns("some-other-type")

							fakePipeline.ResourcesReturns(db.Resources{fakeResource, fakeResource2}, nil)
						})

						It("triggers using the current config", func() {
							Expect(fakeScheduler.TriggerImmediatelyCallCount()).To(Equal(1))

							_, job, resources, resourceTypes := fakeScheduler.TriggerImmediatelyArgsForCall(0)
							Expect(job).To(Equal(fakeJob))
							Expect(resources).To(Equal(db.Resources{fakeResource, fakeResource2}))
							Expect(resourceTypes).To(Equal(versionedResourceTypes))
						})

						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns Content-Type 'application/json'", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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

					Context("when getting the config fails", func() {
						BeforeEach(func() {
							fakePipeline.ResourcesReturns(db.Resources{}, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
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
				})
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
				fakeaccess.IsAuthorizedReturns(true)
				fakeaccess.IsAuthenticatedReturns(true)
			})

			It("looked up the proper pipeline", func() {
				Expect(dbTeam.PipelineCallCount()).To(Equal(1))
				pipelineName := dbTeam.PipelineArgsForCall(0)
				Expect(pipelineName).To(Equal("some-pipeline"))
			})

			Context("when getting the job succeeds", func() {
				var fakeJob *dbfakes.FakeJob

				Context("when it contains the requested job", func() {
					var fakeScheduler *schedulerfakes.FakeBuildScheduler
					BeforeEach(func() {
						fakeJob = new(dbfakes.FakeJob)
						fakeJob.NameReturns("some-job")
						fakeJob.ConfigReturns(atc.JobConfig{
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
						})
						fakePipeline.JobReturns(fakeJob, true, nil)

						fakeScheduler = new(schedulerfakes.FakeBuildScheduler)
						fakeSchedulerFactory.BuildSchedulerReturns(fakeScheduler)

						resource1 := new(dbfakes.FakeResource)
						resource1.NameReturns("some-resource")
						resource1.SourceReturns(atc.Source{"some": "source"})

						resource2 := new(dbfakes.FakeResource)
						resource2.NameReturns("some-other-resource")
						resource2.SourceReturns(atc.Source{"some": "other-source"})
						fakePipeline.ResourcesReturns([]db.Resource{resource1, resource2}, nil)
					})

					Context("when the input versions for the job can be determined", func() {
						BeforeEach(func() {
							fakeJob.GetNextBuildInputsStub = func() ([]db.BuildInput, bool, error) {
								defer GinkgoRecover()
								Expect(fakeScheduler.SaveNextInputMappingCallCount()).To(Equal(1))
								return []db.BuildInput{
									{
										Name: "some-input",
										VersionedResource: db.VersionedResource{
											Resource: "some-resource",
											Type:     "some-type",
											Version:  db.ResourceVersion{"some": "version"},
										},
									},
									{
										Name: "some-other-input",
										VersionedResource: db.VersionedResource{
											Resource: "some-other-resource",
											Type:     "some-other-type",
											Version:  db.ResourceVersion{"some": "other-version"},
										},
									},
								}, true, nil
							}
						})

						It("returns 200 OK", func() {
							Expect(response.StatusCode).To(Equal(http.StatusOK))
						})

						It("returns Content-Type 'application/json'", func() {
							Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
						})

						It("created the scheduler with the correct fakePipeline and external URL", func() {
							actualPipeline, actualExternalURL, actualVariables := fakeSchedulerFactory.BuildSchedulerArgsForCall(0)
							Expect(actualPipeline.Name()).To(Equal(fakePipeline.Name()))
							Expect(actualExternalURL).To(Equal(externalURL))
							Expect(actualVariables).To(Equal(variables))
						})

						It("determined the inputs with the correct job config", func() {
							_, receivedJob, _ := fakeScheduler.SaveNextInputMappingArgsForCall(0)
							Expect(receivedJob.Name()).To(Equal(fakeJob.Name()))
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

						Context("when getting the resources fails", func() {
							BeforeEach(func() {
								fakePipeline.ResourcesReturns(nil, errors.New("some-error"))
							})

							It("returns 500", func() {
								Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the job has no input versions available", func() {
						BeforeEach(func() {
							fakeJob.GetNextBuildInputsReturns(nil, false, nil)
						})

						It("returns 404", func() {
							Expect(response.StatusCode).To(Equal(http.StatusNotFound))
						})
					})

					Context("when the input versions for the job can not be determined", func() {
						BeforeEach(func() {
							fakeJob.GetNextBuildInputsReturns(nil, false, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
						})
					})
				})
			})

			Context("when it does not contain the requested job", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, nil)
				})

				It("returns 404 Not Found", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("when getting the job fails", func() {
				BeforeEach(func() {
					fakePipeline.JobReturns(nil, false, errors.New("some-error"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})

		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns unauthorized", func() {
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

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthorizedReturns(true)
				fakeaccess.IsAuthenticatedReturns(true)
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
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				fakeaccess.IsAuthorizedReturns(false)
			})

			Context("and the pipeline is private", func() {
				BeforeEach(func() {
					fakePipeline.PublicReturns(false)
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthenticatedReturns(false)
					})
					It("returns 401", func() {
						Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
					})
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						fakeaccess.IsAuthenticatedReturns(true)
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
				fakeaccess.IsAuthenticatedReturns(true)
			})
			Context("when authorized", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthorizedReturns(true)

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
				fakeaccess.IsAuthenticatedReturns(false)
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
				fakeaccess.IsAuthorizedReturns(true)
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)

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
				fakeaccess.IsAuthenticatedReturns(false)
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
				fakeaccess.IsAuthorizedReturns(true)
			})

			Context("when authenticated", func() {
				BeforeEach(func() {
					fakeaccess.IsAuthenticatedReturns(true)

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
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
						Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
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
				fakeaccess.IsAuthenticatedReturns(false)
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
