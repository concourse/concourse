package concourse_test

import (
	"net/http"

	"github.com/concourse/concourse/atc"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("ATC Handler Build Resources", func() {
	Describe("BuildResources", func() {
		expectedURL := "/api/v1/builds/6/resources"

		Context("when build exists", func() {
			var expectedBuildInputsOutputs atc.BuildInputsOutputs

			BeforeEach(func() {
				expectedBuildInputsOutputs = atc.BuildInputsOutputs{
					Inputs: []atc.PublicBuildInput{
						{
							Name:    "input1",
							Version: atc.Version{"version": "value1"},
						},
						{
							Name:            "input2",
							Version:         atc.Version{"version": "value2"},
							PipelineID:      57,
							FirstOccurrence: false,
						},
					},
					Outputs: []atc.PublicBuildOutput{
						{
							Name:    "myresource3",
							Version: atc.Version{"version": "value3"},
						},
						{
							Name:    "myresource4",
							Version: atc.Version{"version": "value4"},
						},
					},
				}

				atcServer.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", expectedURL),
						ghttp.RespondWithJSONEncoded(http.StatusOK, expectedBuildInputsOutputs),
					),
				)
			})

			It("returns the inputs and outputs for a given build", func() {
				buildInputsOutputs, found, err := client.BuildResources(6)
				Expect(err).NotTo(HaveOccurred())
				Expect(buildInputsOutputs).To(Equal(expectedBuildInputsOutputs))
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
				_, found, err := client.BuildResources(6)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})
	})
})
