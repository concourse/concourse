package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResourceType", func() {
	Context("when ATC request succeeds", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resource-types/myresource/check"

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			found, err := team.CheckResourceType("mypipeline", "myresource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when pipeline or resource-type does not exist", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resource-types/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("returns a ResourceNotFoundError", func() {
			found, err := team.CheckResourceType("mypipeline", "myresource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Context("when ATC responds with an internal server error", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resource-types/myresource/check"

			atcResponse := atc.CheckResponseBody{
				ExitStatus: 1,
				Stderr:     "internal server error",
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusInternalServerError, atcResponse),
				),
			)
		})

		It("returns an error", func() {
			_, err := team.CheckResourceType("mypipeline", "myresource")
			Expect(err).To(HaveOccurred())

			cre, ok := err.(concourse.CheckResourceError)
			Expect(ok).To(BeTrue())
			Expect(cre.Error()).To(ContainSubstring("internal server error"))
		})
	})

})
