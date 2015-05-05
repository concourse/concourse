package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {

	Describe("GET /api/v1/pipelines", func() {
		var response *http.Response

		BeforeEach(func() {
			pipelinesDB.GetAllActivePipelinesReturns([]db.SavedPipeline{
				{
					ID:     1,
					Paused: false,
					Pipeline: db.Pipeline{
						Name: "a-pipeline",
					},
				},
				{
					ID:     2,
					Paused: true,
					Pipeline: db.Pipeline{
						Name: "another-pipeline",
					},
				},
			}, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/pipelines", nil)
			Ω(err).ShouldNot(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when the call to get active pipelines fails", func() {
			BeforeEach(func() {
				pipelinesDB.GetAllActivePipelinesReturns(nil, errors.New("disaster"))
			})

			It("returns 500 internal server error", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})

		It("returns 200 OK", func() {
			Ω(response.StatusCode).Should(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Ω(response.Header.Get("Content-Type")).Should(Equal("application/json"))
		})

		It("returns all active pipelines", func() {
			body, err := ioutil.ReadAll(response.Body)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(body).Should(MatchJSON(`[
      {
        "name": "a-pipeline",
        "url": "/pipelines/a-pipeline",
				"paused": false
      },{
        "name": "another-pipeline",
        "url": "/pipelines/another-pipeline",
				"paused": true
      }]`))
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/pause", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/pause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Ω(pipelineName).Should(Equal("a-pipeline"))
			})

			Context("when pausing the pipeline succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseReturns(nil)
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when pausing the pipeline fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseReturns(errors.New("welp"))
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

	Describe("PUT /api/v1/pipelines/:pipeline_name/unpause", func() {
		var response *http.Response
		var pipelineDB *dbfakes.FakePipelineDB

		BeforeEach(func() {
			pipelineDB = new(dbfakes.FakePipelineDB)

			pipelineDBFactory.BuildWithNameReturns(pipelineDB, nil)
		})

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/unpause", nil)
			Ω(err).ShouldNot(HaveOccurred())

			response, err = client.Do(request)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Ω(pipelineDBFactory.BuildWithNameCallCount()).Should(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Ω(pipelineName).Should(Equal("a-pipeline"))
			})

			Context("when unpausing the pipeline succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseReturns(nil)
				})

				It("returns 200", func() {
					Ω(response.StatusCode).Should(Equal(http.StatusOK))
				})
			})

			Context("when unpausing the pipeline fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseReturns(errors.New("welp"))
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
