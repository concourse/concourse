package api_test

import (
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

			It("fetches by job", func() {
				Ω(jobsDB.GetJobFinishedAndNextBuildCallCount()).Should(Equal(1))

				jobName := jobsDB.GetJobFinishedAndNextBuildArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns the job's name, url, and any running and finished builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`{
					"name": "some-job",
					"url": "/jobs/some-job",
					"next_build": {
						"id": 3, "name": "2", "job_name": "some-job", "status": "started"
					},
					"finished_build": {
						"id": 1, "name": "1", "job_name": "some-job", "status": "succeeded"
					}
				}`))
			})

			Context("when there are no running or finished builds", func() {
				BeforeEach(func() {
					jobsDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)
				})

				It("returns null as their entries", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{
						"name": "some-job",
						"url": "/jobs/some-job",
						"next_build": null,
						"finished_build": null
					}`))
				})
			})

			Context("when there is no running build", func() {
				BeforeEach(func() {
					jobsDB.GetJobFinishedAndNextBuildReturns(
						&db.Build{
							ID:      1,
							Name:    "1",
							JobName: "some-job",
							Status:  db.StatusSucceeded,
						},
						nil,
						nil,
					)
				})

				It("returns null as its entry", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{
						"name": "some-job",
						"url": "/jobs/some-job",
						"next_build": null,
						"finished_build": {
							"id": 1, "name": "1", "job_name": "some-job", "status": "succeeded"
						}
					}`))
				})
			})

			Context("when there is no finished build", func() {
				BeforeEach(func() {
					jobsDB.GetJobFinishedAndNextBuildReturns(
						nil,
						&db.Build{
							ID:      1,
							Name:    "1",
							JobName: "some-job",
							Status:  db.StatusStarted,
						},
						nil,
					)
				})

				It("returns null as their entries", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{
						"name": "some-job",
						"url": "/jobs/some-job",
						"next_build": {
							"id": 1, "name": "1", "job_name": "some-job", "status": "started"
						},
						"finished_build": null
					}`))
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
					Jobs: []atc.JobConfig{
						{Name: "job-1"},
						{Name: "job-2"},
						{Name: "job-3"},
					},
				}, nil)
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

				It("returns 200 OK", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})

				It("returns each job's name, url, and any running and finished builds", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`[
						{
							"name": "job-1",
							"url": "/jobs/job-1",
							"next_build": {
								"id": 3, "name": "2", "job_name": "job-1", "status": "started"
							},
							"finished_build": {
								"id": 1, "name": "1", "job_name": "job-1", "status": "succeeded"
							}
						},
						{
							"name": "job-2",
							"url": "/jobs/job-2",
							"next_build": null,
							"finished_build": {
								"id": 4, "name": "1", "job_name": "job-2", "status": "succeeded"
							}
						},
						{
							"name": "job-3",
							"url": "/jobs/job-3",
							"next_build": null,
							"finished_build": null
						}
					]`))
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
				configDB.GetConfigReturns(atc.Config{}, errors.New("oh no!"))
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
					{"id": 3, "name": "2", "job_name": "some-job", "status": "started"},
					{"id": 1, "name": "1", "job_name": "some-job", "status": "succeeded"}
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

				Ω(body).Should(MatchJSON(`{"id": 1, "name": "1", "job_name": "some-job", "status": "succeeded"}`))
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
})
