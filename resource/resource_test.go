package resource_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/atc/resource"
	"github.com/concourse/atc/worker"
)

var _ = Describe("Resource", func() {
	Describe("Release", func() {
		It("releases the container", func() {
			resource.Release(worker.FinalTTL(time.Hour))

			Expect(fakeContainer.ReleaseCallCount()).To(Equal(1))
			Expect(fakeContainer.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(time.Hour)))
		})
	})

	Describe("ResourcesDir", func() {
		It("returns a file path with a prefix", func() {
			Expect(ResourcesDir("some-prefix")).To(ContainSubstring("some-prefix"))
		})
	})
})
