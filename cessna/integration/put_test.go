package integration_test

import (
	"github.com/concourse/atc"
	. "github.com/concourse/atc/cessna/resource"
	"github.com/concourse/baggageclaim"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Put a resource", func() {

	var (
		getBaseResource Resource
		getVolume       baggageclaim.Volume
		getErr          error

		putBaseResource Resource
		putResponse     OutResponse
		putErr          error
	)

	Context("whose type is a base resource type", func() {

		BeforeEach(func() {
			source := atc.Source{
				"versions": []map[string]string{
					{"ref": "123"},
					{"beep": "boop"},
				},
			}

			getBaseResource = NewBaseResource(baseResourceType, source)

			getVolume, getErr = ResourceGet{Resource: getBaseResource, Version: atc.Version{"beep": "boop"}}.Get(logger, testWorker)

			putBaseResource = NewBaseResource(baseResourceType, source)
		})

		JustBeforeEach(func() {
			putResponse, putErr = ResourcePut{
				Resource: putBaseResource,
				Params: atc.Params{
					"path": "inputresource/version",
				},
			}.Put(logger, testWorker, NamedArtifacts{
				"inputresource": getVolume,
			})
		})

		It("runs the out script", func() {
			Expect(putErr).ShouldNot(HaveOccurred())
		})

		It("outputs the version that was in the path param file", func() {
			Expect(putResponse.Version).To(Equal(atc.Version{"beep": "boop"}))
		})

	})

})
