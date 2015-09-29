package resource_test

import (
	"errors"

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

			Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
		})
	})

	Describe("Destroy", func() {
		It("destroys the container", func() {
			err := resource.Destroy()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeContainer.DestroyCallCount()).Should(Equal(1))
		})

		It("only destroys on the first call", func() {
			err := resource.Destroy()
			Ω(err).ShouldNot(HaveOccurred())

			err = resource.Destroy()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeContainer.DestroyCallCount()).Should(Equal(1))
		})

		Context("when destroying the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeContainer.DestroyReturns(disaster)
			})

			It("returns the error", func() {
				err := resource.Destroy()
				Ω(err).Should(Equal(disaster))
			})
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
				Ω(err).ShouldNot(HaveOccurred())
				Ω(found).Should(BeTrue())

				Ω(fakeContainer.VolumesCallCount()).Should(Equal(1))

				Ω(volume).Should(Equal(vol1))
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
				Ω(err).Should(Equal(ErrMultipleVolumes))
			})
		})

		Context("when the container has no volumes", func() {
			BeforeEach(func() {
				fakeContainer.VolumesReturns([]baggageclaim.Volume{})
			})

			It("returns no volume and false", func() {
				volume, found, err := resource.CacheVolume()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(found).Should(BeFalse())
				Ω(volume).Should(BeNil())
			})
		})
	})

	Describe("Type", func() {
		It("returns the resource's type", func() {
			Ω(resource.Type()).Should(Equal(ResourceType("some-type")))
		})
	})

	Describe("ResourcesDir", func() {
		It("returns a file path with a prefix", func() {
			Ω(ResourcesDir("some-prefix")).Should(ContainSubstring("some-prefix"))
		})
	})
})
