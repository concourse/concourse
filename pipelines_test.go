package atcclient_test

import (
	"net/http"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipelines", func() {
	var (
		client    atcclient.Client
		handler   atcclient.AtcHandler
		atcServer *ghttp.Server
	)

	BeforeEach(func() {
		var err error
		atcServer = ghttp.NewServer()

		client, err = atcclient.NewClient(
			rc.NewTarget(atcServer.URL(), "", "", "", false),
		)
		Expect(err).NotTo(HaveOccurred())

		handler = atcclient.NewAtcHandler(client)
	})

	AfterEach(func() {
		atcServer.Close()
	})

	Describe("DeletePipeline", func() {
		var (
			expectedURL string
		)

		JustBeforeEach(func() {
			expectedURL = "/api/v1/pipelines/mypipeline"

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

	Describe("Errors", func() {
		Describe("404 Not Found", func() {
			var (
				expectedURL string
			)

			JustBeforeEach(func() {
				expectedURL = "/api/v1/pipelines/mypipeline"

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("deletes the pipeline when called", func() {
				err := handler.DeletePipeline("mypipeline")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("`mypipeline` does not exist"))
			})
		})
	})
})
