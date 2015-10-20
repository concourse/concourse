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
		expectedURL := "/api/v1/pipelines/mypipeline/jobs/myjob/inputs"

		Context("when pipeline/job exists", func() {
			var expectedBuildInputs []atc.BuildInput

			BeforeEach(func() {
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
				buildInputs, found, err := handler.BuildInputsForJob("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(buildInputs).To(Equal(expectedBuildInputs))
				Expect(found).To(BeTrue())
			})
		})

		Context("when pipeline/job does not exist", func() {
			BeforeEach(func() {
				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWith(http.StatusNotFound, ""),
					),
				)
			})

			It("returns false in the found value and no error", func() {
				_, found, err := handler.BuildInputsForJob("mypipeline", "myjob")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
