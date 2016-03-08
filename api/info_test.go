package api_test

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
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

		It("contains the version", func() {
			var info atc.Info

			err := json.NewDecoder(response.Body).Decode(&info)
			Expect(err).NotTo(HaveOccurred())

			Expect(info.Version).To(Equal("1.2.3"))
		})
	})
})
