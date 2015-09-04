package metric_test

import (
	"runtime"
	"sync"

	. "github.com/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counter", func() {
	var counter *Counter

	BeforeEach(func() {
		counter = &Counter{}
	})

	It("tracks the maximum value seen since last checked", func() {
		counter.Inc()
		counter.Inc()
		counter.Dec()

		Expect(counter.Max()).To(Equal(int64(2)))
	})

	It("deals with concurrent increments correctly", func() {
		// buckle up
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(runtime.NumCPU()))

		totalIncs := 30
		wg := new(sync.WaitGroup)
		wg.Add(totalIncs)

		for i := 0; i < totalIncs; i++ {
			go func() {
				counter.Inc()
				wg.Done()
			}()
		}

		wg.Wait()

		Expect(counter.Max()).To(Equal(int64(totalIncs)))
	})

	It("resets the max to the current value when observed", func() {
		counter.Inc()
		counter.Inc()
		counter.Dec()

		Expect(counter.Max()).To(Equal(int64(2)))

		Expect(counter.Max()).To(Equal(int64(1)))
	})
})
