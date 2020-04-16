package wrappa_test

import (
	"sync"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent Request Policy", func() {
	Describe("LimitedRoute#UnmarshalFlag", func() {
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

		It("can acquire a handler", func() {
			pool := pool(1)

			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("fails to acquire a handler when the limit is reached", func() {
			pool := pool(1)

			pool.TryAcquire()
			Expect(pool.TryAcquire()).To(BeFalse())
		})

		It("can acquire a handler after releasing", func() {
			pool := pool(1)

			pool.TryAcquire()
			pool.Release()
			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("can acquire multiple handlers", func() {
			pool := pool(2)

			pool.TryAcquire()
			Expect(pool.TryAcquire()).To(BeTrue())
		})

		It("cannot release more handlers than are held", func() {
			pool := pool(1)

			Expect(pool.Release()).NotTo(Succeed())
		})

		It("cannot acquire multiple handlers simultaneously", func() {
			pool := pool(100)
			failed := false

			doInParallel(101, func() {
				if !pool.TryAcquire() {
					failed = true
				}
			})

			Expect(failed).To(BeTrue())
		})

		It("cannot release multiple handlers simultaneously", func() {
			pool := pool(1000)
			doInParallel(1000, func() { pool.TryAcquire() })
			failed := false

			doInParallel(1001, func() {
				if pool.Release() != nil {
					failed = true
				}
			})

			Expect(failed).To(BeTrue())
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

func pool(size int) wrappa.Pool {
	p, _ := wrappa.NewConcurrentRequestPolicy(
		map[wrappa.LimitedRoute]int{
			wrappa.LimitedRoute(atc.CreateJobBuild): size,
		},
	).HandlerPool(atc.CreateJobBuild)

	return p
}

func doInParallel(numGoroutines int, thingToDo func()) {
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			thingToDo()
			wg.Done()
		}()
	}
	wg.Wait()
}
