package gcng_test

import (
	"errors"

	. "github.com/concourse/atc/gcng"
	"github.com/concourse/atc/gcng/gcngfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Aggregate Collector", func() {
	var (
		subject Collector

		fakeWorkerCollector        *gcngfakes.FakeCollector
		fakeResourceCacheCollector *gcngfakes.FakeCollector
		fakeVolumeCollector        *gcngfakes.FakeCollector

		err      error
		disaster error
	)

	BeforeEach(func() {
		fakeWorkerCollector = new(gcngfakes.FakeCollector)
		fakeResourceCacheCollector = new(gcngfakes.FakeCollector)
		fakeVolumeCollector = new(gcngfakes.FakeCollector)

		subject = NewCollector(
			fakeWorkerCollector,
			fakeVolumeCollector,
			fakeResourceCacheCollector,
		)

		disaster = errors.New("disaster")
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			err = subject.Run()
		})

		It("attempts to collect workers", func() {
			Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
		})

		Context("when the worker collector errors", func() {
			BeforeEach(func() {
				fakeWorkerCollector.RunReturns(disaster)
			})

			It("bubbles up the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
			})
		})

		Context("when the worker collector succeeds", func() {
			It("attempts to collect caches", func() {
				Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
			})

			Context("when the cache collector errors", func() {
				BeforeEach(func() {
					fakeResourceCacheCollector.RunReturns(disaster)
				})

				It("bubbles up the error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(disaster))
				})
			})

			Context("when the cache collector succeeds", func() {
				It("attempts to collect volumes", func() {
					Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
				})

				Context("when the volume collector errors", func() {
					BeforeEach(func() {
						fakeVolumeCollector.RunReturns(disaster)
					})

					It("bubbles up the error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(disaster))
					})
				})

				Context("when the volume collector succeeds", func() {
					It("does not error at all", func() {
						Expect(err).NotTo(HaveOccurred())
					})
				})
			})
		})
	})
})
