package api_test

import (
	"encoding/json"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/api/resources"
	"github.com/concourse/atc/builds"
)

var _ = Describe("Jobs API", func() {
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
					{ID: 2, Status: builds.StatusStarted},
					{ID: 1, Status: builds.StatusSucceeded},
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
				var build []resources.Build
				err := json.NewDecoder(response.Body).Decode(&build)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build).Should(Equal([]resources.Build{
					{ID: 2, Status: "started"},
					{ID: 1, Status: "succeeded"},
				}))
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
				jobsDB.GetJobBuildReturns(builds.Build{ID: 2}, nil)
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
				var build resources.Build
				err := json.NewDecoder(response.Body).Decode(&build)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build).Should(Equal(resources.Build{ID: 2}))
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

	Describe("GET /api/v1/jobs/:job_name/current-build", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/jobs/some-job/current-build")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the build succeeds", func() {
			BeforeEach(func() {
				jobsDB.GetCurrentBuildReturns(builds.Build{ID: 2}, nil)
			})

			It("fetches by job and build name", func() {
				Ω(jobsDB.GetCurrentBuildCallCount()).Should(Equal(1))

				jobName := jobsDB.GetCurrentBuildArgsForCall(0)
				Ω(jobName).Should(Equal("some-job"))
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns the build", func() {
				var build resources.Build
				err := json.NewDecoder(response.Body).Decode(&build)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(build).Should(Equal(resources.Build{ID: 2}))
			})
		})

		Context("when getting the build fails", func() {
			BeforeEach(func() {
				jobsDB.GetCurrentBuildReturns(builds.Build{}, errors.New("oh no!"))
			})

			It("returns 404 Not Found", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusNotFound))
			})
		})
	})
})
