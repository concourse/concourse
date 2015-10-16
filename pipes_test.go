package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Pipes", func() {
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
			pipe, err := handler.CreatePipe()
			Expect(err).NotTo(HaveOccurred())
			Expect(pipe).To(Equal(expectedPipe))
		})
	})
})
