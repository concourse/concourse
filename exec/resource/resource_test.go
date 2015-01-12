package resource_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/exec/resource"
)

var _ = Describe("Resource", func() {
	Describe("Release", func() {
		It("destroys the container", func() {
			err := resource.Release()
			Ω(err).ShouldNot(HaveOccurred())

			Ω(fakeContainer.DestroyCallCount()).Should(Equal(1))
		})

		Context("when destroying the container fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeContainer.DestroyReturns(disaster)
			})

			It("returns the error", func() {
				err := resource.Release()
				Ω(err).Should(Equal(disaster))
			})
		})
	})

	Describe("Type", func() {
		It("returns the resource's type", func() {
			Ω(resource.Type()).Should(Equal(ResourceType("some-type")))
		})
	})
})
