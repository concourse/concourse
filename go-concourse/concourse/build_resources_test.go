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
					Inputs: []atc.PublicBuildInput{
						{
							Name:     "input1",
							Resource: "myresource1",
							Type:     "git",
							Version:  atc.Version{"version": "value1"},
							Metadata: []atc.MetadataField{
								{
									Name:  "meta1",
									Value: "value1",
								},
								{
									Name:  "meta2",
									Value: "value2",
								},
							},
							PipelineID:      57,
							FirstOccurrence: true,
						},
						{
							Name:            "input2",
							Resource:        "myresource2",
							Type:            "git",
							Version:         atc.Version{"version": "value2"},
							Metadata:        []atc.MetadataField{},
							PipelineID:      57,
							FirstOccurrence: false,
						},
					},
					Outputs: []atc.VersionedResource{
						{
							Resource: "myresource3",
							Version:  atc.Version{"version": "value3"},
						},
						{
							Resource: "myresource4",
							Version:  atc.Version{"version": "value4"},
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
