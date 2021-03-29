package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Policy", func() {
	Describe("ConcurrentRequestPolicy#HandlerPool", func() {
		It("returns true when an action is limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[string]int{
					atc.CreateJobBuild: 0,
				},
			)

			_, found := policy.HandlerPool(atc.CreateJobBuild)
			Expect(found).To(BeTrue())
		})

		It("returns false when an action is not limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[string]int{
					atc.CreateJobBuild: 0,
				},
			)

			_, found := policy.HandlerPool(atc.ListAllPipelines)
			Expect(found).To(BeFalse())
		})

		It("holds a reference to its pool", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[string]int{
					atc.CreateJobBuild: 1,
				},
			)
			pool1, _ := policy.HandlerPool(atc.CreateJobBuild)
			pool1.TryAcquire()
			pool2, _ := policy.HandlerPool(atc.CreateJobBuild)
			Expect(pool2.TryAcquire()).To(BeFalse())
		})
	})
})
