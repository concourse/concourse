package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
	"github.com/concourse/baggageclaim"
	bfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Resource", func() {
	Describe("Release", func() {
		It("releases the container", func() {
			resource.Release()

			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
		})
	})

	Describe("CacheVolume", func() {
		Context("when the container has one volume", func() {
			var vol1 *bfakes.FakeVolume

			BeforeEach(func() {
				vol1 = new(bfakes.FakeVolume)
				fakeContainer.VolumesReturns([]baggageclaim.Volume{vol1})
			})

			It("returns the volume and true", func() {
				volume, found, err := resource.CacheVolume()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(fakeContainer.VolumesCallCount()).To(Equal(1))

				Expect(volume).To(Equal(vol1))
			})
		})

		Context("when the container has two volumes", func() {
			var vol1 *bfakes.FakeVolume
			var vol2 *bfakes.FakeVolume

			BeforeEach(func() {
				vol1 = new(bfakes.FakeVolume)
				vol2 = new(bfakes.FakeVolume)
				fakeContainer.VolumesReturns([]baggageclaim.Volume{vol1, vol2})
			})

			It("returns ErrMultipleVolumes", func() {
				_, _, err := resource.CacheVolume()
				Expect(err).To(Equal(ErrMultipleVolumes))
			})
		})

		Context("when the container has no volumes", func() {
			BeforeEach(func() {
				fakeContainer.VolumesReturns([]baggageclaim.Volume{})
			})

			It("returns no volume and false", func() {
				volume, found, err := resource.CacheVolume()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(volume).To(BeNil())
			})
		})
	})

	Describe("ResourcesDir", func() {
		It("returns a file path with a prefix", func() {
			Expect(ResourcesDir("some-prefix")).To(ContainSubstring("some-prefix"))
		})
	})
})
