package concourse_test

import (
	"net/http"

	"github.com/concourse/atc"

	. "github.com/onsi/ginkgo"
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
					Inputs: []atc.VersionedResource{
						{
							Resource: "some-input-resource",
							Version: atc.Version{
								"some": "version",
							},
						},
						{
							Resource: "some-other-input-resource",
							Version: atc.Version{
								"some": "version",
							},
						},
					},
					Outputs: []atc.VersionedResource{
						{
							Resource: "some-output-resource",
							Version: atc.Version{
								"some": "version",
							},
						},
						{
							Resource: "some-other-output-resource",
							Version: atc.Version{
								"some": "version",
							},
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
