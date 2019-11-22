package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/resource"
)

var _ = Describe("Resource", func() {
	Describe("ResourcesDir", func() {
		It("returns a file path with a prefix", func() {
			Expect(ResourcesDir("some-prefix")).To(ContainSubstring("some-prefix"))
		})
	})

	Describe("Signature", func(){
		var (
			resource Resource
			source atc.Source
			params atc.Params
			version atc.Version

			)

		BeforeEach(func(){
			source = atc.Source{ "some-source-key": "some-source-value"}
			params = atc.Params{ "some-params-key": "some-params-value"}
			version = atc.Version{ "some-version-key": "some-version-value"}

			resource = NewResourceFactory().NewResource(source, params, version)
		})

		It("marshals the source, params and version", func(){
			actualSignature, err := resource.Signature()
			Expect(err).ToNot(HaveOccurred())
			Expect(actualSignature).To(MatchJSON(`{
			  "source": {
				"some-source-key": "some-source-value"
			  },
			  "params": {
				"some-params-key": "some-params-value"
			  },
			  "version": {
				"some-version-key": "some-version-value"
			  }
			}`))
		})
	})
})
