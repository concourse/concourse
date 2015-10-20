package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipelines", func() {
	Describe("ListPipelines", func() {
		var expectedPipelines []atc.Pipeline

		BeforeEach(func() {
			expectedURL := "/api/v1/pipelines"

			expectedPipelines = []atc.Pipeline{
				{
					Name:   "mypipeline-1",
					Paused: true,
				},
				{
					Name:   "mypipeline-2",
					Paused: false,
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedPipelines),
				),
			)
		})

		It("returns all the pipelines", func() {
			pipelines, err := handler.ListPipelines()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipelines).To(Equal(expectedPipelines))
		})
	})

	Describe("DeletePipeline", func() {
		expectedURL := "/api/v1/pipelines/mypipeline"

		Context("when the pipeline exists", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNoContent, ""),
					),
				)
			})

			It("deletes the pipeline when called", func() {
				found, err := handler.DeletePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(atcServer.ReceivedRequests()).To(HaveLen(1))
			})
		})

		Context("when the pipeline does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false and no error", func() {
				found, err := handler.DeletePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
