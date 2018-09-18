package metric_test

import (
	"runtime"
	"sync"

	. "github.com/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Gauge", func() {
	var gauge *Gauge

	BeforeEach(func() {
		gauge = &Gauge{}
	})

	It("tracks the maximum value seen since last checked", func() {
		gauge.Inc()
		gauge.Inc()
		gauge.Dec()

		Expect(gauge.Max()).To(Equal(2))
	})

	It("deals with concurrent increments correctly", func() {
		// buckle up
		defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(runtime.NumCPU()))

		totalIncs := 30
		wg := new(sync.WaitGroup)
		wg.Add(totalIncs)

		for i := 0; i < totalIncs; i++ {
			go func() {
				gauge.Inc()
				wg.Done()
			}()
		}

		wg.Wait()

		Expect(gauge.Max()).To(Equal(totalIncs))
	})

	It("resets the max to the current value when observed", func() {
		gauge.Inc()
		gauge.Inc()
		gauge.Dec()

		Expect(gauge.Max()).To(Equal(2))

		Expect(gauge.Max()).To(Equal(1))
	})
})
