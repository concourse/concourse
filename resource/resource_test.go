package resource_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	bfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Resource", func() {
	Describe("Release", func() {
		It("releases the container", func() {
			resource.Release(worker.FinalTTL(time.Hour))

			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
			Expect(fakeContainer.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(time.Hour)))
		})
	})

	Describe("CacheVolume", func() {
		Context("when the container has one volume", func() {
			var vol1 *bfakes.FakeVolume

			BeforeEach(func() {
				vol1 = new(bfakes.FakeVolume)
				fakeContainer.VolumesReturns([]worker.Volume{vol1})
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
				fakeContainer.VolumesReturns([]worker.Volume{vol1, vol2})
			})

			It("returns ErrMultipleVolumes", func() {
				_, _, err := resource.CacheVolume()
				Expect(err).To(Equal(ErrMultipleVolumes))
			})
		})

		Context("when the container has no volumes", func() {
			BeforeEach(func() {
				fakeContainer.VolumesReturns([]worker.Volume{})
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
