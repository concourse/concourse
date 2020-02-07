package metric_test

import (
	. "github.com/concourse/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Counter", func() {
	var counter *Counter

	BeforeEach(func() {
		counter = &Counter{}
	})

	Context("when incremented", func() {
		It("returns incremented value", func() {
			Expect(counter.Delta()).To(Equal(float64(0)))

			counter.Inc()
			counter.Inc()
			counter.Inc()

			Expect(counter.Delta()).To(Equal(float64(3)))
		})
	})

	Context("when incremented by a value", func() {
		It("returns the incremented value", func() {
			Expect(counter.Delta()).To(Equal(float64(0)))

			counter.IncDelta(3)

			Expect(counter.Delta()).To(Equal(float64(3)))
		})
	})

	Context("when current value is requested", func() {
		It("starts counting from 0", func() {
			counter.Inc()
			counter.Inc()
			counter.Inc()

			Expect(counter.Delta()).To(Equal(float64(3)))
			Expect(counter.Delta()).To(Equal(float64(0)))
		})
	})
})
