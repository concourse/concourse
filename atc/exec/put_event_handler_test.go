package exec_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/resource/v2"
)

var _ = Describe("Put Event Handler", func() {
	var handler v2.PutEventHandler

	BeforeEach(func() {
		handler = NewPutEventHandler()
	})

	Describe("CreatedResponse", func() {
		var (
			space       atc.Space
			version     atc.Version
			putResponse atc.PutResponse
			responseErr error
		)

		BeforeEach(func() {
			space = atc.Space("space")
			version = atc.Version{"ref": "v2"}
			putResponse = atc.PutResponse{
				Space: atc.Space("space"),
				CreatedVersions: []atc.Version{
					{"ref": "v1"},
				},
			}
		})

		JustBeforeEach(func() {
			responseErr = handler.CreatedResponse(space, version, &putResponse)
		})

		It("sets the put response", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(putResponse).To(Equal(atc.PutResponse{Space: space, CreatedVersions: []atc.Version{
				{"ref": "v1"},
				version,
			}}))
		})

		Context("when the response contains more than one space", func() {
			BeforeEach(func() {
				putResponse = atc.PutResponse{
					Space: atc.Space("different-space"),
				}
			})

			It("returns multiple spaces error", func() {
				Expect(responseErr).To(HaveOccurred())
				Expect(responseErr).To(Equal(exec.PutMultipleSpacesError{atc.Space("different-space"), space}))
			})
		})
	})
})
