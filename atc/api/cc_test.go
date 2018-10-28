package api_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/api/accessor/accessorfakes"
	"github.com/tedsuo/rata"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cc.xml", func() {
	var (
		requestGenerator *rata.RequestGenerator
		fakeaccess       *accessorfakes.FakeAccess
	)

	BeforeEach(func() {
		requestGenerator = rata.NewRequestGenerator(server.URL, atc.Routes)

		fakeaccess = new(accessorfakes.FakeAccess)
	})

	JustBeforeEach(func() {
		fakeAccessor.CreateReturns(fakeaccess)
	})

	Describe("GET /api/v1/teams/:team_name/cc.xml", func() {
		var response *http.Response

		JustBeforeEach(func() {
			req, err := requestGenerator.CreateRequest(atc.GetCC, rata.Params{
				"team_name":     "a-team",
			}, nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authorized", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(true)
				fakeaccess.IsAuthorizedReturns(true)
			})

			Context("when the team is not found", func() {
				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				fakeaccess.IsAuthenticatedReturns(false)
			})

			It("returns 401", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
