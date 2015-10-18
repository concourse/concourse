package atcclient_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipelines", func() {
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
				err := handler.DeletePipeline("mypipeline")
				Expect(err).NotTo(HaveOccurred())
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

			It("returns an error saying the pipeline does not exist", func() {
				err := handler.DeletePipeline("mypipeline")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("`mypipeline` does not exist"))
			})
		})
	})
})
