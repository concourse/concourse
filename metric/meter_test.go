package metric_test

import (
	. "github.com/concourse/atc/metric"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Meter", func() {
	var meter Meter

	Context("when incremented", func() {
		It("returns incremented value", func() {
			Expect(meter.Delta()).To(Equal(0))

			meter.Inc()
			meter.Inc()
			meter.Inc()

			Expect(meter.Delta()).To(Equal(3))
		})
	})

	Context("when incremented by a value", func() {
		It("returns the incremented value", func() {
			Expect(meter.Delta()).To(Equal(0))

			meter.IncDelta(3)

			Expect(meter.Delta()).To(Equal(3))
		})
	})

	Context("when current value is requested", func() {
		It("starts counting from 0", func() {
			meter.Inc()
			meter.Inc()
			meter.Inc()

			Expect(meter.Delta()).To(Equal(3))
			Expect(meter.Delta()).To(Equal(0))
		})
	})
})
