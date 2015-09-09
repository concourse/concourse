package api_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/algorithm"
	dbfakes "github.com/concourse/atc/db/fakes"
)

var _ = Describe("Jobs API", func() {
	var pipelineDB *dbfakes.FakePipelineDB

	BeforeEach(func() {
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildWithNameReturns(pipelineDB, nil)
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/jobs/:job_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/some-pipeline/jobs/some-job")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
			pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
			Ω(pipelineName).Should(Equal("some-pipeline"))
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{
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
							InputConfigs: []atc.JobInputConfig{
								{
									Resource: "some-input",
								},
								{
									RawName:  "some-name",
									Resource: "some-other-input",
									Params:   atc.Params{"secret": "params"},
									Passed:   []string{"a", "b"},
									Trigger:  true,
								},
							},
							OutputConfigs: []atc.JobOutputConfig{
								{
									Resource: "some-output",
								},
								{
									Resource:     "some-other-output",
									Params:       atc.Params{"secret": "params"},
									RawPerformOn: []atc.Condition{"failure"},
								},
							},
						},
					},
				}, 1, nil)
			})

			Context("when getting the build succeeds", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(
						&db.Build{
							ID:           1,
							Name:         "1",
							JobName:      "some-job",
							PipelineName: "some-pipeline",
							Status:       db.StatusSucceeded,
						},
						&db.Build{
							ID:           3,
							Name:         "2",
							JobName:      "some-job",
							PipelineName: "some-pipeline",
							Status:       db.StatusStarted,
						},
						nil,
					)
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{
							ID:           1,
							Paused:       true,
							PipelineName: "some-pipeline",
							Job: db.Job{
								Name: "job-1",
							},
						}, nil)
					})

					It("fetches by job", func() {
						Ω(pipelineDB.GetJobFinishedAndNextBuildCallCount()).Should(Equal(1))

						jobName := pipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)
						Ω(jobName).Should(Equal("some-job"))
					})

					It("returns 200 OK", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusOK))
					})

					It("returns the job's name, url, if it's paused, and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(body).Should(MatchJSON(`{
							"name": "some-job",
							"paused": true,
							"url": "/pipelines/some-pipeline/jobs/some-job",
							"next_build": {
								"id": 3,
								"name": "2",
								"job_name": "some-job",
								"status": "started",
								"url": "/pipelines/some-pipeline/jobs/some-job/builds/2"
							},
							"finished_build": {
								"id": 1,
								"name": "1",
								"job_name": "some-job",
								"status": "succeeded",
								"url": "/pipelines/some-pipeline/jobs/some-job/builds/1"
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
				})

				Context("when there are no running or finished builds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
					})

					It("returns null as their entries", func() {
						var job atc.Job
						err := json.NewDecoder(response.Body).Decode(&job)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(job.NextBuild).Should(BeNil())
						Ω(job.FinishedBuild).Should(BeNil())
					})
				})
			})

			Context("when getting the job's builds fails", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not present in the config", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{
						Jobs: []atc.JobConfig{
							{Name: "other-job"},
						},
					}, 1, nil)
				})

				It("returns 404", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when getting the job config fails", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/jobs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/some-pipeline/jobs")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name: "group-1",
							Jobs: []string{"job-1"},
						},
						{
							Name: "group-2",
							Jobs: []string{"job-1", "job-2"},
						},
					},

					Jobs: []atc.JobConfig{
						{
							Name:          "job-1",
							InputConfigs:  []atc.JobInputConfig{{Resource: "input-1"}},
							OutputConfigs: []atc.JobOutputConfig{{Resource: "output-1"}},
						},
						{
							Name: "job-2",
							InputConfigs: []atc.JobInputConfig{
								{
									Resource: "input-2",
								},
							},
							OutputConfigs: []atc.JobOutputConfig{{Resource: "output-2"}},
						},
						{
							Name:          "job-3",
							InputConfigs:  []atc.JobInputConfig{{Resource: "input-3"}},
							OutputConfigs: []atc.JobOutputConfig{{Resource: "output-3"}},
						},
					},
				}, 1, nil)
			})

			Context("when getting each job's builds succeeds", func() {
				BeforeEach(func() {
					call := 0

					pipelineDB.GetJobFinishedAndNextBuildStub = func(jobName string) (*db.Build, *db.Build, error) {
						call++

						var finishedBuild, nextBuild *db.Build

						switch call {
						case 1:
							Ω(jobName).Should(Equal("job-1"))

							finishedBuild = &db.Build{
								ID:           1,
								Name:         "1",
								JobName:      jobName,
								PipelineName: "another-pipeline",
								Status:       db.StatusSucceeded,
							}

							nextBuild = &db.Build{
								ID:           3,
								Name:         "2",
								JobName:      jobName,
								PipelineName: "another-pipeline",
								Status:       db.StatusStarted,
							}

						case 2:
							Ω(jobName).Should(Equal("job-2"))

							finishedBuild = &db.Build{
								ID:           4,
								Name:         "1",
								JobName:      "job-2",
								PipelineName: "another-pipeline",
								Status:       db.StatusSucceeded,
							}

						case 3:
							Ω(jobName).Should(Equal("job-3"))

						default:
							panic("unexpected call count")
						}

						return finishedBuild, nextBuild, nil
					}
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						pipelineDB.GetJobReturns(db.SavedJob{
							ID:           1,
							Paused:       true,
							PipelineName: "another-pipeline",
							Job: db.Job{
								Name: "job-1",
							},
						}, nil)
					})

					It("returns 200 OK", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusOK))
					})

					It("returns each job's name, url, and any running and finished builds", func() {
						body, err := ioutil.ReadAll(response.Body)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(body).Should(MatchJSON(`[
							{
								"name": "job-1",
								"paused": true,
								"url": "/pipelines/another-pipeline/jobs/job-1",
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"url": "/pipelines/another-pipeline/jobs/job-1/builds/2"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"url": "/pipelines/another-pipeline/jobs/job-1/builds/1"
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": false}],
								"outputs": [{"name": "output-1", "resource": "output-1"}],
								"groups": ["group-1", "group-2"]
							},
							{
								"name": "job-2",
								"paused": true,
								"url": "/pipelines/another-pipeline/jobs/job-2",
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"url": "/pipelines/another-pipeline/jobs/job-2/builds/1"
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": false}],
								"outputs": [{"name": "output-2", "resource": "output-2"}],
								"groups": ["group-2"]
							},
							{
								"name": "job-3",
								"paused": true,
								"url": "/pipelines/another-pipeline/jobs/job-3",
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": false}],
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					pipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the job config fails", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/jobs/:job_name/builds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/some-pipeline/jobs/some-job/builds")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
			pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
			Ω(pipelineName).Should(Equal("some-pipeline"))
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetAllJobBuildsReturns([]db.Build{
					{
						ID:           3,
						Name:         "2",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						Status:       db.StatusStarted,
					},
					{
						ID:           1,
						Name:         "1",
						JobName:      "some-job",
						PipelineName: "some-pipeline",
						Status:       db.StatusSucceeded,
					},
				}, nil)
			})

			It("fetches by job and build name", func() {
				Ω(pipelineDB.GetAllJobBuildsCallCount()).Should(Equal(1))

				jobName := pipelineDB.GetAllJobBuildsArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns the builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`[
					{
						"id": 3,
						"name": "2",
						"job_name": "some-job",
						"status": "started",
						"url": "/pipelines/some-pipeline/jobs/some-job/builds/2"
					},
					{
						"id": 1,
						"name": "1",
						"job_name": "some-job",
						"status": "succeeded",
						"url": "/pipelines/some-pipeline/jobs/some-job/builds/1"
					}
				]`))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				pipelineDB.GetAllJobBuildsReturns(nil, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/jobs/:job_name/inputs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/some-pipeline/jobs/some-job/inputs")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("looked up the proper pipeline", func() {
				Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Ω(pipelineName).Should(Equal("some-pipeline"))
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

					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{
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
						}, 42, nil)
					})

					Context("when the versions can be loaded", func() {
						versionsDB := &algorithm.VersionsDB{}

						BeforeEach(func() {
							pipelineDB.LoadVersionsDBReturns(versionsDB, nil)
						})

						Context("when the input versions for the job can be determined", func() {
							BeforeEach(func() {
								pipelineDB.GetLatestInputVersionsReturns([]db.BuildInput{
									{
										Name: "some-input",
										VersionedResource: db.VersionedResource{
											Resource:     "some-resource",
											Type:         "some-type",
											Version:      db.Version{"some": "version"},
											PipelineName: "some-pipeline",
										},
									},
									{
										Name: "some-other-input",
										VersionedResource: db.VersionedResource{
											Resource:     "some-other-resource",
											Type:         "some-other-type",
											Version:      db.Version{"some": "other-version"},
											PipelineName: "some-pipeline",
										},
									},
								}, nil)
							})

							It("returns 200 OK", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusOK))
							})

							It("determined the inputs with the correct versions DB, job name, and inputs", func() {
								receivedVersionsDB, receivedJob, receivedInputs := pipelineDB.GetLatestInputVersionsArgsForCall(0)
								Expect(receivedVersionsDB).To(Equal(versionsDB))
								Expect(receivedJob).To(Equal("some-job"))
								Expect(receivedInputs).To(Equal(config.JobInputs(someJob)))
							})

							It("returns the inputs", func() {
								body, err := ioutil.ReadAll(response.Body)
								Ω(err).ShouldNot(HaveOccurred())

								Ω(body).Should(MatchJSON(`[
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
								pipelineDB.GetLatestInputVersionsReturns(nil, db.ErrNoVersions)
							})

							It("returns 404", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
							})
						})

						Context("when the input versions for the job can not be determined", func() {
							BeforeEach(func() {
								pipelineDB.GetLatestInputVersionsReturns(nil, errors.New("oh no!"))
							})

							It("returns 500", func() {
								Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
							})
						})
					})

					Context("when the versions can not be loaded", func() {
						BeforeEach(func() {
							pipelineDB.LoadVersionsDBReturns(nil, errors.New("oh no!"))
						})

						It("returns 500", func() {
							Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
						})
					})
				})

				Context("when it does not contain the requested job", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{
							Jobs: atc.JobConfigs{
								{
									Name: "some-bogus-job",
									Plan: atc.PlanSequence{},
								},
							},
						}, 42, nil)
					})

					It("returns 404 Not Found", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
					})
				})
			})

			Context("when getting the config fails", func() {
				Context("with ErrPipelineNotFound", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, db.ErrPipelineNotFound)
					})

					It("returns 404", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
					})
				})

				Context("with an unknown error", func() {
					BeforeEach(func() {
						pipelineDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/jobs/:job_name/builds/:build_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/some-pipeline/jobs/some-job/builds/some-build")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
			pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
			Ω(pipelineName).Should(Equal("some-pipeline"))
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildReturns(db.Build{
					ID:           1,
					Name:         "1",
					JobName:      "some-job",
					PipelineName: "a-pipeline",
					Status:       db.StatusSucceeded,
				}, nil)
			})

			It("fetches by job and build name", func() {
				Ω(pipelineDB.GetJobBuildCallCount()).Should(Equal(1))

				jobName, buildName := pipelineDB.GetJobBuildArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
				Ω(buildName).Should(Equal("some-build"))
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns the build", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`{
					"id": 1,
					"name": "1",
					"job_name": "some-job",
					"status": "succeeded",
					"url": "/pipelines/a-pipeline/jobs/some-job/builds/1"
				}`))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				pipelineDB.GetJobBuildReturns(db.Build{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/jobs/:job_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/some-pipeline/jobs/job-name/pause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the PipelineDB", func() {
				Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Ω(pipelineName).Should(Equal("some-pipeline"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Ω(pipelineDB.PauseJobArgsForCall(0)).Should(Equal("job-name"))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseJobReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/jobs/:job_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/some-pipeline/jobs/job-name/unpause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the PipelineDB", func() {
				Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Ω(pipelineName).Should(Equal("some-pipeline"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Ω(pipelineDB.UnpauseJobArgsForCall(0)).Should(Equal("job-name"))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseJobReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusUnauthorized))
			})
		})
	})
})
