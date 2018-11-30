package exec_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/resource/v2"
)

var _ = Describe("Put Event Handler", func() {
	var handler v2.PutEventHandler

	BeforeEach(func() {
		handler = exec.NewPutEventHandler()
	})

	Describe("CreatedResponse", func() {
		var (
			space    atc.Space
			version  atc.Version
			metadata atc.Metadata

			previousVersions []atc.SpaceVersion
			actualVersions   []atc.SpaceVersion

			responseErr error
		)

		BeforeEach(func() {
			space = atc.Space("space")
			version = atc.Version{"ref": "v2"}
			metadata = atc.Metadata{
				atc.MetadataField{
					Name:  "meta",
					Value: "data2",
				},
			}

			previousVersions = []atc.SpaceVersion{
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"ref": "v1"},
					Metadata: atc.Metadata{
						atc.MetadataField{
							Name:  "meta",
							Value: "data",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			actualVersions, responseErr = handler.CreatedResponse(space, version, metadata, previousVersions)
		})

		It("appends the version to the list", func() {
			Expect(responseErr).ToNot(HaveOccurred())
			Expect(actualVersions).To(ConsistOf([]atc.SpaceVersion{
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"ref": "v1"},
					Metadata: atc.Metadata{
						atc.MetadataField{
							Name:  "meta",
							Value: "data",
						},
					},
				},
				atc.SpaceVersion{
					Space:   atc.Space("space"),
					Version: atc.Version{"ref": "v2"},
					Metadata: atc.Metadata{
						atc.MetadataField{
							Name:  "meta",
							Value: "data2",
						},
					},
				},
			}))
		})

		Context("when the response contains more than one space", func() {
			BeforeEach(func() {
				previousVersions = []atc.SpaceVersion{
					atc.SpaceVersion{
						Space:   atc.Space("different-space"),
						Version: atc.Version{"ref": "v1"},
						Metadata: atc.Metadata{
							atc.MetadataField{
								Name:  "meta",
								Value: "data",
							},
						},
					},
				}
			})

			It("returns multiple spaces error", func() {
				Expect(responseErr).To(HaveOccurred())
				Expect(responseErr).To(Equal(exec.PutMultipleSpacesError{atc.Space("different-space"), space}))
			})
		})
	})
})
