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

var _ = Describe("ATC Handler Build Inputs", func() {
	var (
		handler   atcclient.AtcHandler
		atcServer *ghttp.Server
		client    atcclient.Client
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

	Describe("BuildInputsForJob", func() {

		var (
			expectedBuildInputs []atc.BuildInput
			expectedURL         string
		)

		BeforeEach(func() {
			expectedURL = "/api/v1/pipelines/mypipeline/jobs/myjob/inputs"

			expectedBuildInputs = []atc.BuildInput{
				{
					Name:     "myfirstinput",
					Resource: "myfirstinput",
				},
				{
					Name:     "mySecondinput",
					Resource: "mySecondinput",
				},
			}

			atcServer.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", expectedURL),
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuildInputs, http.Header{}),
				),
			)
		})

		It("returns the input configuration for the given job", func() {
			buildInputs, err := handler.BuildInputsForJob("mypipeline", "myjob")
			Expect(err).NotTo(HaveOccurred())
			Expect(buildInputs).To(Equal(expectedBuildInputs))
		})
	})
})
