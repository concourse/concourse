package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/builds"
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
					&builds.Build{
						ID:      1,
						Name:    "1",
						JobName: "some-job",
						Status:  builds.StatusSucceeded,
					},
					&builds.Build{
						ID:      3,
						Name:    "2",
						JobName: "some-job",
						Status:  builds.StatusStarted,
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

			It("returns running and finished builds", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`{
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
						"next_build": null,
						"finished_build": null
					}`))
				})
			})

			Context("when there is no running build", func() {
				BeforeEach(func() {
					jobsDB.GetJobFinishedAndNextBuildReturns(
						&builds.Build{
							ID:      1,
							Name:    "1",
							JobName: "some-job",
							Status:  builds.StatusSucceeded,
						},
						nil,
						nil,
					)
				})

				It("returns null as its entry", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{
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
						&builds.Build{
							ID:      1,
							Name:    "1",
							JobName: "some-job",
							Status:  builds.StatusStarted,
						},
						nil,
					)
				})

				It("returns null as their entries", func() {
					body, err := ioutil.ReadAll(response.Body)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(body).Should(MatchJSON(`{
						"next_build": {
							"id": 1, "name": "1", "job_name": "some-job", "status": "started"
						},
						"finished_build": null
					}`))
				})
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				jobsDB.GetJobFinishedAndNextBuildReturns(nil, nil, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
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
				jobsDB.GetAllJobBuildsReturns([]builds.Build{
					{
						ID:      3,
						Name:    "2",
						JobName: "some-job",
						Status:  builds.StatusStarted,
					},
					{
						ID:      1,
						Name:    "1",
						JobName: "some-job",
						Status:  builds.StatusSucceeded,
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
				jobsDB.GetJobBuildReturns(builds.Build{
					ID:      1,
					Name:    "1",
					JobName: "some-job",
					Status:  builds.StatusSucceeded,
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
				jobsDB.GetJobBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})
})
