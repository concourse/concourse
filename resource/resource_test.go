package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
)

var _ = Describe("Resource", func() {
	Describe("Release", func() {
		It("releases the container", func() {
			Expect(fakeContainer.ReleaseCallCount()).To(Equal(0))
			resource.Release()
			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
		})
	})

	Describe("ResourcesDir", func() {
		It("returns a file path with a prefix", func() {
			Expect(ResourcesDir("some-prefix")).To(ContainSubstring("some-prefix"))
		})
	})
})
