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

		fakeWorkerCollector            *gcngfakes.FakeCollector
		fakeResourceCacheUseCollector  *gcngfakes.FakeCollector
		fakeResourceConfigUseCollector *gcngfakes.FakeCollector
		fakeResourceConfigCollector    *gcngfakes.FakeCollector
		fakeResourceCacheCollector     *gcngfakes.FakeCollector
		fakeVolumeCollector            *gcngfakes.FakeCollector
		fakeContainerCollector         *gcngfakes.FakeCollector

		err      error
		disaster error
	)

	BeforeEach(func() {
		fakeWorkerCollector = new(gcngfakes.FakeCollector)
		fakeResourceCacheUseCollector = new(gcngfakes.FakeCollector)
		fakeResourceConfigUseCollector = new(gcngfakes.FakeCollector)
		fakeResourceConfigCollector = new(gcngfakes.FakeCollector)
		fakeResourceCacheCollector = new(gcngfakes.FakeCollector)
		fakeVolumeCollector = new(gcngfakes.FakeCollector)
		fakeContainerCollector = new(gcngfakes.FakeCollector)

		subject = NewCollector(
			fakeWorkerCollector,
			fakeResourceCacheUseCollector,
			fakeResourceConfigUseCollector,
			fakeResourceConfigCollector,
			fakeResourceCacheCollector,
			fakeVolumeCollector,
			fakeContainerCollector,
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

			It("bubbles up the error and stops", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(disaster))
				Expect(fakeResourceCacheUseCollector.RunCallCount()).To(BeZero())
			})
		})

		Context("when the worker collector succeeds", func() {
			It("attempts to collect cache uses", func() {
				Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
			})

			Context("when the cache use collector errors", func() {
				BeforeEach(func() {
					fakeResourceCacheUseCollector.RunReturns(disaster)
				})

				It("bubbles up the error and stops", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(disaster))
					Expect(fakeResourceConfigUseCollector.RunCallCount()).To(BeZero())
				})
			})

			Context("when the cache use collector succeeds", func() {
				It("attempts to collect config uses", func() {
					Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
				})

				Context("when the config use collector errors", func() {
					BeforeEach(func() {
						fakeResourceConfigUseCollector.RunReturns(disaster)
					})

					It("bubbles up the error and stops", func() {
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(disaster))
						Expect(fakeResourceConfigCollector.RunCallCount()).To(BeZero())
					})
				})

				Context("when the config use collector succeeds", func() {
					It("attempts to collect configs", func() {
						Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
					})

					Context("when the config collector errors", func() {
						BeforeEach(func() {
							fakeResourceConfigCollector.RunReturns(disaster)
						})

						It("bubbles up the error and stops", func() {
							Expect(err).To(HaveOccurred())
							Expect(err).To(Equal(disaster))
							Expect(fakeResourceCacheCollector.RunCallCount()).To(BeZero())
						})
					})

					Context("when the config use collector succeeds", func() {
						It("attempts to collect caches", func() {
							Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
						})

						Context("when the cache collector errors", func() {
							BeforeEach(func() {
								fakeResourceConfigCollector.RunReturns(disaster)
							})

							It("bubbles up the error and stops", func() {
								Expect(err).To(HaveOccurred())
								Expect(err).To(Equal(disaster))
								Expect(fakeVolumeCollector.RunCallCount()).To(BeZero())
							})
						})

						Context("when the config use collector succeeds", func() {
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
								It("attempts to collect containers", func() {
									Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
								})

								Context("when the container collector errors", func() {
									BeforeEach(func() {
										fakeContainerCollector.RunReturns(disaster)
									})

									It("bubbles up the error", func() {
										Expect(err).To(HaveOccurred())
										Expect(err).To(Equal(disaster))
									})
								})

								Context("when the container collector succeeds", func() {
									It("does not error at all", func() {
										Expect(err).NotTo(HaveOccurred())
									})
								})
							})
						})
					})
				})
			})
		})
	})
})
