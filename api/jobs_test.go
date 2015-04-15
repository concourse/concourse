package api_test

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
)

var _ = Describe("Jobs API", func() {
	Describe("GET /api/v1/jobs/:job_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/jobs/some-job")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				no := false

				configDB.GetConfigReturns(atc.Config{
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
									RawName:    "some-name",
									Resource:   "some-other-input",
									Params:     atc.Params{"secret": "params"},
									Passed:     []string{"a", "b"},
									RawTrigger: &no,
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
					jobsDB.GetJobFinishedAndNextBuildReturns(
						&db.Build{
							ID:      1,
							Name:    "1",
							JobName: "some-job",
							Status:  db.StatusSucceeded,
						},
						&db.Build{
							ID:      3,
							Name:    "2",
							JobName: "some-job",
							Status:  db.StatusStarted,
						},
						nil,
					)
				})

				Context("when getting the job fails", func() {
					BeforeEach(func() {
						jobsDB.GetJobReturns(db.Job{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})
				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						jobsDB.GetJobReturns(db.Job{
							Name:   "job-1",
							Paused: true,
						}, nil)
					})

					It("fetches by job", func() {
						Ω(jobsDB.GetJobFinishedAndNextBuildCallCount()).Should(Equal(1))

						jobName := jobsDB.GetJobFinishedAndNextBuildArgsForCall(0)
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
							"url": "/jobs/some-job",
							"next_build": {
								"id": 3,
								"name": "2",
								"job_name": "some-job",
								"status": "started",
								"url": "/jobs/some-job/builds/2"
							},
							"finished_build": {
								"id": 1,
								"name": "1",
								"job_name": "some-job",
								"status": "succeeded",
								"url": "/jobs/some-job/builds/1"
							},
							"inputs": [
								{
									"name": "some-input",
									"resource": "some-input",
									"trigger": true
								},
								{
									"name": "some-name",
									"resource": "some-other-input",
									"passed": ["a", "b"],
									"trigger": false
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
						jobsDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
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
					jobsDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})

			Context("when the job is not present in the config", func() {
				BeforeEach(func() {
					configDB.GetConfigReturns(atc.Config{
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
				configDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/jobs", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/jobs")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the job config succeeds", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{
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

					jobsDB.GetJobFinishedAndNextBuildStub = func(jobName string) (*db.Build, *db.Build, error) {
						call++

						var finishedBuild, nextBuild *db.Build

						switch call {
						case 1:
							Ω(jobName).Should(Equal("job-1"))

							finishedBuild = &db.Build{
								ID:      1,
								Name:    "1",
								JobName: jobName,
								Status:  db.StatusSucceeded,
							}

							nextBuild = &db.Build{
								ID:      3,
								Name:    "2",
								JobName: jobName,
								Status:  db.StatusStarted,
							}

						case 2:
							Ω(jobName).Should(Equal("job-2"))

							finishedBuild = &db.Build{
								ID:      4,
								Name:    "1",
								JobName: "job-2",
								Status:  db.StatusSucceeded,
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
						jobsDB.GetJobReturns(db.Job{}, errors.New("nope"))
					})

					It("returns 500", func() {
						Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
					})
				})

				Context("when getting the job succeeds", func() {
					BeforeEach(func() {
						jobsDB.GetJobReturns(db.Job{
							Name:   "job-1",
							Paused: true,
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
								"url": "/jobs/job-1",
								"next_build": {
									"id": 3,
									"name": "2",
									"job_name": "job-1",
									"status": "started",
									"url": "/jobs/job-1/builds/2"
								},
								"finished_build": {
									"id": 1,
									"name": "1",
									"job_name": "job-1",
									"status": "succeeded",
									"url": "/jobs/job-1/builds/1"
								},
								"inputs": [{"name": "input-1", "resource": "input-1", "trigger": true}],
								"outputs": [{"name": "output-1", "resource": "output-1"}],
								"groups": ["group-1", "group-2"]
							},
							{
								"name": "job-2",
								"paused": true,
								"url": "/jobs/job-2",
								"next_build": null,
								"finished_build": {
									"id": 4,
									"name": "1",
									"job_name": "job-2",
									"status": "succeeded",
									"url": "/jobs/job-2/builds/1"
								},
								"inputs": [{"name": "input-2", "resource": "input-2", "trigger": true}],
								"outputs": [{"name": "output-2", "resource": "output-2"}],
								"groups": ["group-2"]
							},
							{
								"name": "job-3",
								"paused": true,
								"url": "/jobs/job-3",
								"next_build": null,
								"finished_build": null,
								"inputs": [{"name": "input-3", "resource": "input-3", "trigger": true}],
								"outputs": [{"name": "output-3", "resource": "output-3"}],
								"groups": []
							}
						]`))
					})
				})
			})

			Context("when getting the build fails", func() {
				BeforeEach(func() {
					jobsDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the job config fails", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{}, 0, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})

	Describe("GET /api/v1/jobs/:job_name/builds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/jobs/some-job/builds")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				jobsDB.GetAllJobBuildsReturns([]db.Build{
					{
						ID:      3,
						Name:    "2",
						JobName: "some-job",
						Status:  db.StatusStarted,
					},
					{
						ID:      1,
						Name:    "1",
						JobName: "some-job",
						Status:  db.StatusSucceeded,
					},
				}, nil)
			})

			It("fetches by job and build name", func() {
				Ω(jobsDB.GetAllJobBuildsCallCount()).Should(Equal(1))

				jobName := jobsDB.GetAllJobBuildsArgsForCall(0)
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
						"url": "/jobs/some-job/builds/2"
					},
					{
						"id": 1,
						"name": "1",
						"job_name": "some-job",
						"status": "succeeded",
						"url": "/jobs/some-job/builds/1"
					}
				]`))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				jobsDB.GetAllJobBuildsReturns(nil, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("GET /api/v1/jobs/:job_name/builds/:build_name", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/jobs/some-job/builds/some-build")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				jobsDB.GetJobBuildReturns(db.Build{
					ID:      1,
					Name:    "1",
					JobName: "some-job",
					Status:  db.StatusSucceeded,
				}, nil)
			})

			It("fetches by job and build name", func() {
				Ω(jobsDB.GetJobBuildCallCount()).Should(Equal(1))

				jobName, buildName := jobsDB.GetJobBuildArgsForCall(0)
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
					"url": "/jobs/some-job/builds/1"
				}`))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				jobsDB.GetJobBuildReturns(db.Build{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})

	Describe("PUT /api/v1/jobs/:job_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/jobs/job-name/pause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					jobsDB.PauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Ω(jobsDB.PauseJobArgsForCall(0)).Should(Equal("job-name"))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					jobsDB.PauseJobReturns(errors.New("welp"))
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

	Describe("PUT /api/v1/jobs/:job_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/jobs/job-name/unpause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					jobsDB.UnpauseJobReturns(nil)
				})

				It("paused the right job", func() {
					Ω(jobsDB.UnpauseJobArgsForCall(0)).Should(Equal("job-name"))
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when pausing the job fails", func() {
				BeforeEach(func() {
					jobsDB.UnpauseJobReturns(errors.New("welp"))
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
