package api_test

import (
	"io/ioutil"
	"net/http"

	"github.com/concourse/atc/api/accessor/accessorfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = FDescribe("Pipelines API", func() {
	var (
		fakeaccess *accessorfakes.FakeAccess
	)
	BeforeEach(func() {
		fakeaccess = new(accessorfakes.FakeAccess)
	})
	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("GET /api/v1/health/creds", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			req, err := http.NewRequest("GET", server.URL+"/api/v1/health/creds", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		BeforeEach(func() {
			fakeaccess.IsAuthenticatedReturns(true)
			fakeaccess.IsAdminReturns(true)
		})

		It("returns Content-Type 'application/json'", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

		It("contains the version", func() {
			body, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())

			Expect(body).To(MatchJSON(`["this", "is", "your", "creds"]`))
		})
	})
})
