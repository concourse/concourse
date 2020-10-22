package atc_test

import (
	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionedResourceTypes", func() {
	var types atc.VersionedResourceTypes

	BeforeEach(func() {
		types = atc.VersionedResourceTypes{
			{
				ResourceType: atc.ResourceType{
					Name:   "some-type",
					Type:   "some-base-type",
					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:   "nested-type",
					Type:   "some-type",
					Source: atc.Source{"nested": "source"},
				},
				Version: atc.Version{"nested": "version"},
			},
			{
				ResourceType: atc.ResourceType{
					Name:   "overridden-base-type",
					Type:   "overridden-base-type",
					Source: atc.Source{"overriding": "source"},
				},
				Version: atc.Version{"overriding": "version"},
			},
		}
	})

	Describe("Base", func() {
		Context("when the type is not present", func() {
			It("returns the given type", func() {
				Expect(types.Base("bogus")).To(Equal("bogus"))
			})
		})

		Context("when the type has a base type", func() {
			It("returns the base type", func() {
				Expect(types.Base("some-type")).To(Equal("some-base-type"))
			})
		})

		Context("when the type is nested", func() {
			It("returns the bottom type", func() {
				Expect(types.Base("nested-type")).To(Equal("some-base-type"))
			})
		})

		Context("when the type overrides a base type", func() {
			It("returns the base type", func() {
				Expect(types.Base("overridden-base-type")).To(Equal("overridden-base-type"))
			})
		})
	})
})
