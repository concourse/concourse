package wrappa_test

import (
	"sync"

	"github.com/concourse/concourse/atc/wrappa"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	It("can acquire a resource", func() {
		pool := pool(1)

		Expect(pool.TryAcquire()).To(BeTrue())
	})

	It("fails to acquire a resource when the limit is reached", func() {
		pool := pool(1)

		pool.TryAcquire()
		Expect(pool.TryAcquire()).To(BeFalse())
	})

	It("can acquire a resource after releasing", func() {
		pool := pool(1)

		pool.TryAcquire()
		pool.Release()
		Expect(pool.TryAcquire()).To(BeTrue())
	})

	It("can acquire multiple resources", func() {
		pool := pool(2)

		pool.TryAcquire()
		Expect(pool.TryAcquire()).To(BeTrue())
	})

	It("cannot release more resources than are held", func() {
		pool := pool(1)

		Expect(pool.Release).To(Panic())
	})

	It("cannot acquire multiple resources simultaneously", func() {
		pool := pool(100)
		failed := false

		doInParallel(101, func() {
			if !pool.TryAcquire() {
				failed = true
			}
		})

		Expect(failed).To(BeTrue())
	})
})

func pool(size int) wrappa.Pool {
	return wrappa.NewPool(size)
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
