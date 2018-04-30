package api_test

import (
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pipelines API", func() {
	Describe("GET /api/v1/info", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/info")
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Content-Type 'application/json'", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("contains the version", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`{
				"version": "1.2.3",
				"worker_version": "4.5.6"
			}`))
		})
	})
})
