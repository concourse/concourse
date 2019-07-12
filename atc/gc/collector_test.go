package gc_test

import (
	"context"
	"errors"

	. "github.com/concourse/concourse/atc/gc"
	"github.com/concourse/concourse/atc/gc/gcfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Aggregate Collector", func() {
	var (
		subject Collector

		fakeBuildCollector                      *gcfakes.FakeCollector
		fakeWorkerCollector                     *gcfakes.FakeCollector
		fakeResourceCacheUseCollector           *gcfakes.FakeCollector
		fakeResourceConfigCollector             *gcfakes.FakeCollector
		fakeResourceCacheCollector              *gcfakes.FakeCollector
		fakeArtifactCollector                   *gcfakes.FakeCollector
		fakeCheckCollector                      *gcfakes.FakeCollector
		fakeVolumeCollector                     *gcfakes.FakeCollector
		fakeContainerCollector                  *gcfakes.FakeCollector
		fakeResourceConfigCheckSessionCollector *gcfakes.FakeCollector

		err      error
		disaster error
	)

	BeforeEach(func() {
		fakeBuildCollector = new(gcfakes.FakeCollector)
		fakeWorkerCollector = new(gcfakes.FakeCollector)
		fakeResourceCacheUseCollector = new(gcfakes.FakeCollector)
		fakeResourceConfigCollector = new(gcfakes.FakeCollector)
		fakeResourceCacheCollector = new(gcfakes.FakeCollector)
		fakeArtifactCollector = new(gcfakes.FakeCollector)
		fakeCheckCollector = new(gcfakes.FakeCollector)
		fakeVolumeCollector = new(gcfakes.FakeCollector)
		fakeContainerCollector = new(gcfakes.FakeCollector)
		fakeResourceConfigCheckSessionCollector = new(gcfakes.FakeCollector)

		subject = NewCollector(
			fakeBuildCollector,
			fakeWorkerCollector,
			fakeResourceCacheUseCollector,
			fakeResourceConfigCollector,
			fakeResourceCacheCollector,
			fakeArtifactCollector,
			fakeCheckCollector,
			fakeVolumeCollector,
			fakeContainerCollector,
			fakeResourceConfigCheckSessionCollector,
		)

		disaster = errors.New("disaster")
	})

	Describe("Run", func() {
		JustBeforeEach(func() {
			err = subject.Run(context.TODO())
		})

		It("runs the collector", func() {
			Expect(fakeBuildCollector.RunCallCount()).To(Equal(1))
		})

		Context("when the collector errors", func() {
			BeforeEach(func() {
				fakeBuildCollector.RunReturns(disaster)
			})

			It("does not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("runs the rest of collectors", func() {
				Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
				Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
				Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
				Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
				Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
				Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
			})
		})

		Context("when the build collector succeeds", func() {
			It("attempts to collect", func() {
				Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
			})

			Context("when the collector errors", func() {
				BeforeEach(func() {
					fakeWorkerCollector.RunReturns(disaster)
				})

				It("does not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("runs the rest of collectors", func() {
					Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
					Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
					Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
					Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
					Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
					Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
				})
			})

			Context("when the worker collector succeeds", func() {
				It("attempts to collect", func() {
					Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
				})

				Context("when the collector errors", func() {
					BeforeEach(func() {
						fakeResourceCacheUseCollector.RunReturns(disaster)
					})

					It("does not return an error", func() {
						Expect(err).NotTo(HaveOccurred())
					})

					It("runs the rest of collectors", func() {
						Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
						Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
						Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
						Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
						Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
						Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
					})
				})

				Context("when the cache use collector succeeds", func() {
					It("attempts to collect", func() {
						Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
					})

					Context("when the collector errors", func() {
						BeforeEach(func() {
							fakeResourceConfigCollector.RunReturns(disaster)
						})

						It("does not return an error", func() {
							Expect(err).NotTo(HaveOccurred())
						})

						It("runs the rest of collectors", func() {
							Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
							Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
							Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
							Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
							Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
							Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
						})
					})

					Context("when the config collector succeeds", func() {
						It("attempts to collect", func() {
							Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
						})

						Context("when the collector errors", func() {
							BeforeEach(func() {
								fakeResourceCacheCollector.RunReturns(disaster)
							})

							It("does not return an error", func() {
								Expect(err).NotTo(HaveOccurred())
							})

							It("runs the rest of collectors", func() {
								Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
								Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
								Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
								Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
								Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
								Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
							})
						})

						Context("when the config use collector succeeds", func() {
							It("attempts to collect", func() {
								Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
							})

							Context("when the collector errors", func() {
								BeforeEach(func() {
									fakeVolumeCollector.RunReturns(disaster)
								})

								It("does not return an error", func() {
									Expect(err).NotTo(HaveOccurred())
								})

								It("runs the rest of collectors", func() {
									Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
									Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
									Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
									Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
									Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
								})
							})

							Context("when the volume collector succeeds", func() {
								It("attempts to collect", func() {
									Expect(fakeContainerCollector.RunCallCount()).To(Equal(1))
								})

								Context("when the collector errors", func() {
									BeforeEach(func() {
										fakeContainerCollector.RunReturns(disaster)
									})

									It("does not return an error", func() {
										Expect(err).NotTo(HaveOccurred())
									})

									It("runs the rest of collectors", func() {
										Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
										Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
										Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
										Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
										Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
									})
								})
								Context("when the resource config check session collector succeeds", func() {
									It("attempts to collect", func() {
										Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
									})

									Context("when the collector errors", func() {
										BeforeEach(func() {
											fakeResourceConfigCheckSessionCollector.RunReturns(disaster)
										})

										It("does not return an error", func() {
											Expect(err).NotTo(HaveOccurred())
										})

										It("runs the rest of collectors", func() {
											Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
											Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
											Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
											Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
											Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
										})
									})

									Context("when the artifact collector succeeds", func() {
										It("attempts to collect", func() {
											Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
										})

										Context("when the collector errors", func() {
											BeforeEach(func() {
												fakeArtifactCollector.RunReturns(disaster)
											})

											It("does not return an error", func() {
												Expect(err).NotTo(HaveOccurred())
											})

											It("runs the rest of collectors", func() {
												Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
												Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
												Expect(fakeVolumeCollector.RunCallCount()).To(Equal(1))
											})
										})
									})

									Context("when the check collector succeeds", func() {
										It("attempts to collect", func() {
											Expect(fakeCheckCollector.RunCallCount()).To(Equal(1))
										})

										Context("when the collector errors", func() {
											BeforeEach(func() {
												fakeCheckCollector.RunReturns(disaster)
											})

											It("does not return an error", func() {
												Expect(err).NotTo(HaveOccurred())
											})

											It("runs the rest of collectors", func() {
												Expect(fakeWorkerCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceCacheUseCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceConfigCollector.RunCallCount()).To(Equal(1))
												Expect(fakeArtifactCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceCacheCollector.RunCallCount()).To(Equal(1))
												Expect(fakeResourceConfigCheckSessionCollector.RunCallCount()).To(Equal(1))
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
})
