package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {

	Describe("GET /api/v1/pipelines", func() {
		var response *http.Response

		BeforeEach(func() {
			pipelinesDB.GetAllActivePipelinesReturns([]db.SavedPipeline{
				{
					ID: 1,
					Pipeline: db.Pipeline{
						Name: "a-pipeline",
					},
				},
				{
					ID: 2,
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
        "url": "/pipelines/a-pipeline"
      },{
        "name": "another-pipeline",
        "url": "/pipelines/another-pipeline"
      }]`))
		})
	})
})
