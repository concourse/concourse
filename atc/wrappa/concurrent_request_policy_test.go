package wrappa_test

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Policy", func() {
	Describe("LimitedRoute#UnmarshalFlag", func() {
		It("unmarshals ListAllJobs", func() {
			var flagValue wrappa.LimitedRoute
			flagValue.UnmarshalFlag(atc.ListAllJobs)
			expected := wrappa.LimitedRoute(atc.ListAllJobs)
			Expect(flagValue).To(Equal(expected))
		})

		It("raises an error when the action is not supported", func() {
			var flagValue wrappa.LimitedRoute
			err := flagValue.UnmarshalFlag(atc.CreateJobBuild)

			expected := "action 'CreateJobBuild' is not supported"
			Expect(err.Error()).To(ContainSubstring(expected))
		})

		It("error message describes supported actions", func() {
			var flagValue wrappa.LimitedRoute
			err := flagValue.UnmarshalFlag(atc.CreateJobBuild)

			expected := "Supported actions are: "
			Expect(err.Error()).To(ContainSubstring(expected))
		})
	})

	Describe("ConcurrentRequestPolicy#HandlerPool", func() {
		It("returns true when an action is limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[wrappa.LimitedRoute]int{
					wrappa.LimitedRoute(atc.CreateJobBuild): 0,
				},
			)

			_, found := policy.HandlerPool(atc.CreateJobBuild)
			Expect(found).To(BeTrue())
		})

		It("returns false when an action is not limited", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[wrappa.LimitedRoute]int{
					wrappa.LimitedRoute(atc.CreateJobBuild): 0,
				},
			)

			_, found := policy.HandlerPool(atc.ListAllPipelines)
			Expect(found).To(BeFalse())
		})

		It("holds a reference to its pool", func() {
			policy := wrappa.NewConcurrentRequestPolicy(
				map[wrappa.LimitedRoute]int{
					wrappa.LimitedRoute(atc.CreateJobBuild): 1,
				},
			)
			pool1, _ := policy.HandlerPool(atc.CreateJobBuild)
			pool1.TryAcquire()
			pool2, _ := policy.HandlerPool(atc.CreateJobBuild)
			Expect(pool2.TryAcquire()).To(BeFalse())
		})
	})
})
