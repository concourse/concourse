package resource_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
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

	Describe("VolumeHandles", func() {
		Context("when the concourse-volumes property is present", func() {
			BeforeEach(func() {
				fakeContainer.VolumeHandlesReturns([]string{"handle-1", "handle-2"}, nil)
			})

			It("returns the container's volume handles", func() {
				volumes, err := resource.VolumeHandles()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(fakeContainer.VolumeHandlesCallCount()).Should(Equal(1))

				Ω(volumes).Should(Equal([]string{"handle-1", "handle-2"}))
			})
		})

		Context("when getting the volumes fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeContainer.VolumeHandlesReturns(nil, disaster)
			})

			It("returns the error", func() {
				_, err := resource.VolumeHandles()
				Ω(err).Should(Equal(disaster))
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
