package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/onsi/gomega/ghttp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetCheck", func() {
	Context("when ATC request succeeds", func() {
		var expectedCheck atc.Check

		BeforeEach(func() {
			expectedCheck = atc.Check{
				ID:         123,
				Status:     "errored",
				CreateTime: 100000000000,
				StartTime:  100000000000,
				EndTime:    100000000000,
				CheckError: "some-error",
			}

			expectedURL := "/api/v1/checks/123"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedCheck),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			check, found, err := client.Check("123")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(check).To(Equal(expectedCheck))

			Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when check does not exist", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/checks/100"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("returns a ResourceNotFoundError", func() {
			_, found, err := client.Check("100")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Context("when ATC responds with an error", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/checks/123"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWith(http.StatusBadRequest, "bad request"),
				),
			)
		})

		It("returns an error", func() {
			_, _, err := client.Check("123")
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("bad request"))
		})
	})

	Context("when ATC responds with an internal server error", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/checks/123"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWith(http.StatusInternalServerError, "unknown server error"),
				),
			)
		})

		It("returns an error with body", func() {
			_, _, err := client.Check("123")
			Expect(err).To(HaveOccurred())

			cre, ok := err.(concourse.GenericError)
			Expect(ok).To(BeTrue())
			Expect(cre.Error()).To(Equal("unknown server error"))
		})
	})
})
