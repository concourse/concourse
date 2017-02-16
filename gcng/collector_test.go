package gcng_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	. "github.com/concourse/atc/gcng"
	"github.com/concourse/atc/gcng/gcngfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Aggregate Collector", func() {
	var (
		subject Collector

		fakeBuildCollector             *gcngfakes.FakeCollector
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
		logger := lagertest.NewTestLogger("collector")
		fakeBuildCollector = new(gcngfakes.FakeCollector)
		fakeWorkerCollector = new(gcngfakes.FakeCollector)
		fakeResourceCacheUseCollector = new(gcngfakes.FakeCollector)
		fakeResourceConfigUseCollector = new(gcngfakes.FakeCollector)
		fakeResourceConfigCollector = new(gcngfakes.FakeCollector)
		fakeResourceCacheCollector = new(gcngfakes.FakeCollector)
		fakeVolumeCollector = new(gcngfakes.FakeCollector)
		fakeContainerCollector = new(gcngfakes.FakeCollector)

		subject = NewCollector(
			logger,
			fakeBuildCollector,
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

		It("runs the build collector", func() {
			Expect(fakeBuildCollector.RunCallCount()).To(Equal(1))
		})

		Context("when the build collector errors", func() {
			BeforeEach(func() {
				fakeBuildCollector.RunReturns(disaster)
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs the rest of collectors", func() {
				Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
				Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
				Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
			})

		})

		Context("when the build collector succeeds", func() {
			It("attempts to collect workers", func() {
				Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
			})

			Context("when the worker collector errors", func() {
				BeforeEach(func() {
					fakeWorkerCollector.RunReturns(disaster)
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("runs the rest of collectors", func() {
					Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
					Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
					Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
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

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("runs the rest of collectors", func() {
						Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
						Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
						Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
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

						It("does not return an error", func() {
							Expect(err).NotTo(HaveOccurred())
						})

						It("runs the rest of collectors", func() {
							Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
							Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
							Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
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

							It("does not return an error", func() {
								Expect(err).NotTo(HaveOccurred())
							})

							It("runs the rest of collectors", func() {
								Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
								Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
								Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
							})
						})

						Context("when the config use collector succeeds", func() {
							It("attempts to collect caches", func() {
								Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
							})

							Context("when the cache collector errors", func() {
								BeforeEach(func() {
									fakeResourceCacheCollector.RunReturns(disaster)
								})

								It("does not return an error", func() {
									Expect(err).NotTo(HaveOccurred())
								})

								It("runs the rest of collectors", func() {
									Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
									Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
									Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
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

									It("does not return an error", func() {
										Expect(err).NotTo(HaveOccurred())
									})

									It("runs the rest of collectors", func() {
										Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
										Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
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

										It("does not return an error", func() {
											Expect(err).NotTo(HaveOccurred())
										})

										It("runs the rest of collectors", func() {
											Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceConfigUseCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
											Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
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
})
