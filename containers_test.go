package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Containers", func() {
	Describe("ListContainers", func() {
		var (
			expectedContainers []atc.Container
		)

		BeforeEach(func() {
			expectedURL := "/api/v1/containers"

			expectedContainers = []atc.Container{
				{
					ID:           "myid-1",
					PipelineName: "mypipeline-1",
				},
				{
					ID:           "myid-2",
					PipelineName: "mypipeline-2",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedContainers),
				),
			)
		})

		It("returns all the containers", func() {
			containers, err := handler.ListContainers()
			Expect(err).NotTo(HaveOccurred())
			Expect(containers).To(Equal(expectedContainers))
		})
	})
})
