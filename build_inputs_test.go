package atcclient_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Build Inputs", func() {
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
					ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuildInputs),
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
