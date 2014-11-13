package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
)

var _ = Describe("Resources API", func() {
	Describe("GET /api/v1/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/resources")
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when getting the resource config succeeds", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{
					Resources: []atc.ResourceConfig{
						{Name: "resource-1", Type: "type-1"},
						{Name: "resource-2", Type: "type-2"},
						{Name: "resource-3", Type: "type-3"},
					},
				}, nil)
			})

			It("returns 200 OK", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusOK))
			})

			It("returns each resource's name and type", func() {
				body, err := ioutil.ReadAll(response.Body)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(body).Should(MatchJSON(`[
					{
						"name": "resource-1",
						"type": "type-1"
					},
					{
						"name": "resource-2",
						"type": "type-2"
					},
					{
						"name": "resource-3",
						"type": "type-3"
					}
				]`))
			})
		})

		Context("when getting the resource config fails", func() {
			BeforeEach(func() {
				configDB.GetConfigReturns(atc.Config{}, errors.New("oh no!"))
			})

			It("returns 500", func() {
				Ω(response.StatusCode).Should(Equal(http.StatusInternalServerError))
			})
		})
	})
})
