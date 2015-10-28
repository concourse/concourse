package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipes", func() {
	Describe("CreatePipe", func() {
		var (
			expectedURL  string
			expectedPipe atc.Pipe
		)

		BeforeEach(func() {
			expectedURL = "/api/v1/pipes"
			expectedPipe = atc.Pipe{
				ID: "foo",
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusCreated, expectedPipe),
				),
			)
		})

		It("Creates the Pipe when called", func() {
			pipe, err := client.CreatePipe()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipe).To(Equal(expectedPipe))
		})
	})
})
