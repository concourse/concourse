package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/go-concourse/concourse"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResource", func() {
	Context("when ATC request succeeds", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, ""),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			found, err := team.CheckResource("mypipeline", "myresource", atc.Version{"ref": "fake-ref"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when pipeline or resource does not exist", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/check"
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("returns a ResourceNotFoundError", func() {
			found, err := team.CheckResource("mypipeline", "myresource", atc.Version{"ref": "fake-ref"})
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Context("when ATC responds with an error", func() {
		BeforeEach(func() {
			expectedURL := "/api/v1/teams/some-team/pipelines/mypipeline/resources/myresource/check"

			atcResponse := atc.CheckResponseBody{
				ExitStatus: 1,
				Stderr:     "bad version",
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"}}`),
					ghttp.RespondWithJSONEncoded(http.StatusBadRequest, atcResponse),
				),
			)
		})

		It("returns an error", func() {
			_, err := team.CheckResource("mypipeline", "myresource", atc.Version{"ref": "fake-ref"})
			Expect(err).To(HaveOccurred())

			cre, ok := err.(concourse.CheckResourceError)
			Expect(ok).To(BeTrue())
			Expect(cre.Error()).To(Equal("check failed with exit status '1':\nbad version\n"))
		})
	})
})
