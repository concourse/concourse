package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("CheckResourceType", func() {
	var (
		expectedURL   = "/api/v1/teams/some-team/pipelines/mypipeline/resource-types/myresource/check"
		expectedQuery = "vars.branch=%22master%22"
		pipelineRef   = atc.PipelineRef{Name: "mypipeline", InstanceVars: atc.InstanceVars{"branch": "master"}}
	)

	Context("when ATC request succeeds", func() {
		var expectedCheck atc.Build

		BeforeEach(func() {
			expectedCheck = atc.Build{
				ID:        123,
				Status:    "started",
				StartTime: 100000000000,
				EndTime:   100000000000,
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQuery),
					ghttp.VerifyJSON(`{"from":{"ref":"fake-ref"},"shallow":true}`),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedCheck),
				),
			)
		})

		It("sends check resource request to ATC", func() {
			check, found, err := team.CheckResourceType(pipelineRef, "myresource", atc.Version{"ref": "fake-ref"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(check).To(Equal(expectedCheck))

			Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
		})
	})

	Context("when pipeline or resource-type does not exist", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQuery),
					ghttp.RespondWithJSONEncoded(http.StatusNotFound, ""),
				),
			)
		})

		It("returns a ResourceNotFoundError", func() {
			_, found, err := team.CheckResourceType(pipelineRef, "myresource", atc.Version{"ref": "fake-ref"}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Context("when ATC responds with an internal server error", func() {
		BeforeEach(func() {
			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL, expectedQuery),
					ghttp.RespondWith(http.StatusInternalServerError, "generic error"),
				),
			)
		})

		It("returns an error", func() {
			_, _, err := team.CheckResourceType(pipelineRef, "myresource", atc.Version{"ref": "fake-ref"}, true)
			Expect(err).To(HaveOccurred())

			cre, ok := err.(concourse.GenericError)
			Expect(ok).To(BeTrue())
			Expect(cre.Error()).To(Equal("generic error"))
		})
	})

})
