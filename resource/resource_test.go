package resource_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
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
		Context("when the container has a volume mount for /tmp/build/get", func() {
			var vol1 *wfakes.FakeVolume
			var vol2 *wfakes.FakeVolume

			BeforeEach(func() {
				vol1 = new(wfakes.FakeVolume)
				vol2 = new(wfakes.FakeVolume)
				fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
					{
						Volume:    vol1,
						MountPath: "/tmp/build/get",
					},
					{
						Volume:    vol2,
						MountPath: "/tmp/build/forgetaboutit",
					},
				})
			})

			It("returns the volume and true", func() {
				volume, found := resource.CacheVolume()
				Expect(found).To(BeTrue())

				Expect(fakeContainer.VolumeMountsCallCount()).To(Equal(1))

				Expect(volume).To(Equal(vol1))
			})
		})

		Context("when the container does not have a volume mount for /tmp/build/get", func() {
			var vol1 *wfakes.FakeVolume

			BeforeEach(func() {
				vol1 = new(wfakes.FakeVolume)
				fakeContainer.VolumeMountsReturns([]worker.VolumeMount{
					{
						Volume:    vol1,
						MountPath: "/tmp/build/forgetaboutit",
					},
				})
			})

			It("returns no volume and false", func() {
				volume, found := resource.CacheVolume()
				Expect(volume).To(BeNil())
				Expect(found).To(BeFalse())

				Expect(fakeContainer.VolumeMountsCallCount()).To(Equal(1))
			})
		})

		Context("when the container has no volumes", func() {
			BeforeEach(func() {
				fakeContainer.VolumeMountsReturns([]worker.VolumeMount{})
			})

			It("returns no volume and false", func() {
				volume, found := resource.CacheVolume()
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
