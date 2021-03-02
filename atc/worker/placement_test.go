package worker_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db"
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter . ContainerPlacementStrategy

var (
	strategy ContainerPlacementStrategy

	spec     ContainerSpec
	metadata db.ContainerMetadata
	workers  []Worker

	chosenWorker Worker
	chooseErr    error

	newStrategyError error

	compatibleWorkerOneCache1 *workerfakes.FakeWorker
	compatibleWorkerOneCache2 *workerfakes.FakeWorker
	compatibleWorkerTwoCaches *workerfakes.FakeWorker
	compatibleWorkerNoCaches1 *workerfakes.FakeWorker
	compatibleWorkerNoCaches2 *workerfakes.FakeWorker

	logger *lagertest.TestLogger
)

var _ = Describe("FewestBuildContainersPlacementStrategy", func() {
	Describe("Choose", func() {
		var compatibleWorker1 *workerfakes.FakeWorker
		var compatibleWorker2 *workerfakes.FakeWorker
		var compatibleWorker3 *workerfakes.FakeWorker

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("build-containers-equal-placement-test")
			strategy, newStrategyError = NewContainerPlacementStrategy(ContainerPlacementStrategyOptions{ContainerPlacementStrategy: []string{"fewest-build-containers"}})
			Expect(newStrategyError).ToNot(HaveOccurred())
			compatibleWorker1 = new(workerfakes.FakeWorker)
			compatibleWorker1.NameReturns("compatibleWorker1")
			compatibleWorker2 = new(workerfakes.FakeWorker)
			compatibleWorker2.NameReturns("compatibleWorker2")
			compatibleWorker3 = new(workerfakes.FakeWorker)
			compatibleWorker3.NameReturns("compatibleWorker3")

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},

				TeamID: 4567,

				Inputs: []InputSource{},
			}
		})

		Context("when there is only one worker", func() {
			BeforeEach(func() {
				workers = []Worker{compatibleWorker1}
				compatibleWorker1.BuildContainersReturns(20)
			})

			It("picks that worker", func() {
				chosenWorker, chooseErr = strategy.Choose(
					logger,
					workers,
					spec,
				)
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(compatibleWorker1))
			})
		})

		Context("when there are multiple workers", func() {
			BeforeEach(func() {
				workers = []Worker{compatibleWorker1, compatibleWorker2, compatibleWorker3}

				compatibleWorker1.BuildContainersReturns(30)
				compatibleWorker2.BuildContainersReturns(20)
				compatibleWorker3.BuildContainersReturns(10)
			})

			Context("when the container is not of type 'check'", func() {
				It("picks the one with least amount of containers", func() {
					Consistently(func() Worker {
						chosenWorker, chooseErr = strategy.Choose(
							logger,
							workers,
							spec,
						)
						Expect(chooseErr).ToNot(HaveOccurred())
						return chosenWorker
					}).Should(Equal(compatibleWorker3))
				})

				Context("when there is more than one worker with the same number of build containers", func() {
					BeforeEach(func() {
						workers = []Worker{compatibleWorker1, compatibleWorker2, compatibleWorker3}
						compatibleWorker1.BuildContainersReturns(10)
					})

					It("picks any of them", func() {
						Consistently(func() Worker {
							chosenWorker, chooseErr = strategy.Choose(
								logger,
								workers,
								spec,
							)
							Expect(chooseErr).ToNot(HaveOccurred())
							return chosenWorker
						}).Should(Or(Equal(compatibleWorker1), Equal(compatibleWorker3)))
					})
				})

			})
		})
	})
})

var _ = Describe("VolumeLocalityPlacementStrategy", func() {
	Describe("Choose", func() {
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				logger,
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("volume-locality-placement-test")
			strategy, newStrategyError = NewContainerPlacementStrategy(ContainerPlacementStrategyOptions{ContainerPlacementStrategy: []string{"volume-locality"}})
			Expect(newStrategyError).ToNot(HaveOccurred())

			fakeInput1 := new(workerfakes.FakeInputSource)
			fakeInput1AS := new(workerfakes.FakeArtifactSource)
			fakeInput1AS.ExistsOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
				switch worker {
				case compatibleWorkerOneCache1, compatibleWorkerOneCache2, compatibleWorkerTwoCaches:
					return new(workerfakes.FakeVolume), true, nil
				default:
					return nil, false, nil
				}
			}
			fakeInput1.SourceReturns(fakeInput1AS)

			fakeInput2 := new(workerfakes.FakeInputSource)
			fakeInput2AS := new(workerfakes.FakeArtifactSource)
			fakeInput2AS.ExistsOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
				switch worker {
				case compatibleWorkerTwoCaches:
					return new(workerfakes.FakeVolume), true, nil
				default:
					return nil, false, nil
				}
			}
			fakeInput2.SourceReturns(fakeInput2AS)

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},

				TeamID: 4567,

				Inputs: []InputSource{
					fakeInput1,
					fakeInput2,
				},
			}

			compatibleWorkerOneCache1 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache1.SatisfiesReturns(true)
			compatibleWorkerOneCache1.NameReturns("compatibleWorkerOneCache1")

			compatibleWorkerOneCache2 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache2.SatisfiesReturns(true)
			compatibleWorkerOneCache2.NameReturns("compatibleWorkerOneCache2")

			compatibleWorkerTwoCaches = new(workerfakes.FakeWorker)
			compatibleWorkerTwoCaches.SatisfiesReturns(true)
			compatibleWorkerTwoCaches.NameReturns("compatibleWorkerTwoCaches")

			compatibleWorkerNoCaches1 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches1.SatisfiesReturns(true)
			compatibleWorkerNoCaches1.NameReturns("compatibleWorkerNoCaches1")

			compatibleWorkerNoCaches2 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches2.SatisfiesReturns(true)
			compatibleWorkerNoCaches2.NameReturns("compatibleWorkerNoCaches2")
		})

		Context("with one having the most local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerOneCache1,
					compatibleWorkerTwoCaches,
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on the worker with the most caches", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(Equal(compatibleWorkerTwoCaches))
			})
		})

		Context("with multiple with the same amount of local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerOneCache1,
					compatibleWorkerOneCache2,
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on a random one of the two", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerOneCache1), Equal(compatibleWorkerOneCache2)))

				workerChoiceCounts := map[Worker]int{}

				for i := 0; i < 100; i++ {
					worker, err := strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerOneCache1), Equal(compatibleWorkerOneCache2)))
					workerChoiceCounts[worker]++
				}

				Expect(workerChoiceCounts[compatibleWorkerOneCache1]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerOneCache2]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).To(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).To(BeZero())
			})
		})

		Context("with none having any local caches", func() {
			BeforeEach(func() {
				workers = []Worker{
					compatibleWorkerNoCaches1,
					compatibleWorkerNoCaches2,
				}
			})

			It("creates it on a random one of them", func() {
				Expect(chooseErr).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))

				workerChoiceCounts := map[Worker]int{}

				for i := 0; i < 100; i++ {
					worker, err := strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))
					workerChoiceCounts[worker]++
				}

				Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).ToNot(BeZero())
				Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).ToNot(BeZero())
			})
		})
	})
})

var _ = Describe("No strategy should equal to random strategy", func() {
	Describe("Choose", func() {
		JustBeforeEach(func() {
			chosenWorker, chooseErr = strategy.Choose(
				logger,
				workers,
				spec,
			)
		})

		BeforeEach(func() {
			strategy = NewRandomPlacementStrategy()

			workers = []Worker{
				compatibleWorkerNoCaches1,
				compatibleWorkerNoCaches2,
			}
		})

		It("creates it on a random one of them", func() {
			Expect(chooseErr).ToNot(HaveOccurred())
			Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))

			workerChoiceCounts := map[Worker]int{}

			for i := 0; i < 100; i++ {
				worker, err := strategy.Choose(
					logger,
					workers,
					spec,
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(chosenWorker).To(SatisfyAny(Equal(compatibleWorkerNoCaches1), Equal(compatibleWorkerNoCaches2)))
				workerChoiceCounts[worker]++
			}

			Expect(workerChoiceCounts[compatibleWorkerNoCaches1]).ToNot(BeZero())
			Expect(workerChoiceCounts[compatibleWorkerNoCaches2]).ToNot(BeZero())
		})
	})
})

var _ = Describe("LimitActiveContainersPlacementStrategyNode", func() {
	Describe("Choose", func() {
		var compatibleWorker1 *workerfakes.FakeWorker
		var compatibleWorker2 *workerfakes.FakeWorker
		var compatibleWorker3 *workerfakes.FakeWorker
		var activeContainerLimit int

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("build-containers-equal-placement-test")
			compatibleWorker1 = new(workerfakes.FakeWorker)
			compatibleWorker1.NameReturns("compatibleWorker1")
			compatibleWorker2 = new(workerfakes.FakeWorker)
			compatibleWorker2.NameReturns("compatibleWorker2")
			compatibleWorker3 = new(workerfakes.FakeWorker)
			compatibleWorker3.NameReturns("compatibleWorker3")
			activeContainerLimit = 0

			compatibleWorker1.ActiveContainersReturns(20)
			compatibleWorker2.ActiveContainersReturns(200)
			compatibleWorker3.ActiveContainersReturns(200000000000)
			workers = []Worker{compatibleWorker1, compatibleWorker2, compatibleWorker3}

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},
				TeamID:    4567,
				Inputs:    []InputSource{},
			}
		})

		JustBeforeEach(func() {
			strategy, newStrategyError = NewContainerPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy:   []string{"limit-active-containers"},
				MaxActiveContainersPerWorker: activeContainerLimit,
			},
			)
			Expect(newStrategyError).ToNot(HaveOccurred())
		})

		Context("when there is no limit", func() {
			BeforeEach(func() {
				activeContainerLimit = 0
			})

			It("return all workers", func() {
				Consistently(func() Worker {
					chosenWorker, chooseErr = strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(chooseErr).ToNot(HaveOccurred())
					return chosenWorker
				}).Should(Or(Equal(compatibleWorker1), Equal(compatibleWorker2), Equal(compatibleWorker3)))
			})
		})

		Context("when there is a limit", func() {
			Context("when the limit is 20", func() {
				BeforeEach(func() {
					activeContainerLimit = 20
				})

				It("picks worker1", func() {
					Consistently(func() Worker {
						chosenWorker, chooseErr = strategy.Choose(
							logger,
							workers,
							spec,
						)
						Expect(chooseErr).ToNot(HaveOccurred())
						return chosenWorker
					}).Should(Equal(compatibleWorker1))
				})

				Context("when the limit is 200", func() {
					BeforeEach(func() {
						activeContainerLimit = 200
					})

					It("picks worker1 or worker2", func() {
						Consistently(func() Worker {
							chosenWorker, chooseErr = strategy.Choose(
								logger,
								workers,
								spec,
							)
							Expect(chooseErr).ToNot(HaveOccurred())
							return chosenWorker
						}).Should(Or(Equal(compatibleWorker1), Equal(compatibleWorker2)))
					})
				})

				Context("when the limit is too low", func() {
					BeforeEach(func() {
						activeContainerLimit = 1
					})

					It("return no worker", func() {
						chosenWorker, chooseErr = strategy.Choose(
							logger,
							workers,
							spec,
						)
						Expect(chooseErr).To(HaveOccurred())
						Expect(chooseErr).To(Equal(NoWorkerFitContainerPlacementStrategyError{Strategy: "limit-active-containers"}))
						Expect(chosenWorker).To(BeNil())
					})
				})
			})
		})
	})
})

var _ = Describe("LimitActiveVolumesPlacementStrategyNode", func() {
	Describe("Choose", func() {
		var compatibleWorker1 *workerfakes.FakeWorker
		var compatibleWorker2 *workerfakes.FakeWorker
		var compatibleWorker3 *workerfakes.FakeWorker
		var activeVolumeLimit int

		BeforeEach(func() {
			logger = lagertest.NewTestLogger("build-containers-equal-placement-test")
			compatibleWorker1 = new(workerfakes.FakeWorker)
			compatibleWorker1.NameReturns("compatibleWorker1")
			compatibleWorker2 = new(workerfakes.FakeWorker)
			compatibleWorker2.NameReturns("compatibleWorker2")
			compatibleWorker3 = new(workerfakes.FakeWorker)
			compatibleWorker3.NameReturns("compatibleWorker3")
			activeVolumeLimit = 0

			compatibleWorker1.ActiveVolumesReturns(20)
			compatibleWorker2.ActiveVolumesReturns(200)
			compatibleWorker3.ActiveVolumesReturns(200000000000)
			workers = []Worker{compatibleWorker1, compatibleWorker2, compatibleWorker3}

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},
				TeamID:    4567,
				Inputs:    []InputSource{},
			}
		})

		JustBeforeEach(func() {
			strategy, newStrategyError = NewContainerPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"limit-active-volumes"},
				MaxActiveVolumesPerWorker:  activeVolumeLimit,
			},
			)
			Expect(newStrategyError).ToNot(HaveOccurred())
		})

		Context("when there is no limit", func() {
			BeforeEach(func() {
				activeVolumeLimit = 0
			})

			It("return all workers", func() {
				Consistently(func() Worker {
					chosenWorker, chooseErr = strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(chooseErr).ToNot(HaveOccurred())
					return chosenWorker
				}).Should(Or(Equal(compatibleWorker1), Equal(compatibleWorker2), Equal(compatibleWorker3)))
			})
		})

		Context("when there is a limit", func() {
			Context("when the limit is 20", func() {
				BeforeEach(func() {
					activeVolumeLimit = 20
				})

				It("picks worker1", func() {
					Consistently(func() Worker {
						chosenWorker, chooseErr = strategy.Choose(
							logger,
							workers,
							spec,
						)
						Expect(chooseErr).ToNot(HaveOccurred())
						return chosenWorker
					}).Should(Equal(compatibleWorker1))
				})

				Context("when the limit is 200", func() {
					BeforeEach(func() {
						activeVolumeLimit = 200
					})

					It("picks worker1 or worker2", func() {
						Consistently(func() Worker {
							chosenWorker, chooseErr = strategy.Choose(
								logger,
								workers,
								spec,
							)
							Expect(chooseErr).ToNot(HaveOccurred())
							return chosenWorker
						}).Should(Or(Equal(compatibleWorker1), Equal(compatibleWorker2)))
					})
				})

				Context("when the limit is too low", func() {
					BeforeEach(func() {
						activeVolumeLimit = 1
					})

					It("return no worker", func() {
						chosenWorker, chooseErr = strategy.Choose(
							logger,
							workers,
							spec,
						)
						Expect(chooseErr).To(HaveOccurred())
						Expect(chooseErr).To(Equal(NoWorkerFitContainerPlacementStrategyError{Strategy: "limit-active-volumes"}))
						Expect(chosenWorker).To(BeNil())
					})
				})
			})
		})
	})
})

var _ = Describe("ChainedPlacementStrategy #Choose", func() {

	var someWorker1 *workerfakes.FakeWorker
	var someWorker2 *workerfakes.FakeWorker
	var someWorker3 *workerfakes.FakeWorker

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("build-containers-equal-placement-test")
		strategy, newStrategyError = NewContainerPlacementStrategy(
			ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"fewest-build-containers", "volume-locality"},
			})
		Expect(newStrategyError).ToNot(HaveOccurred())
		someWorker1 = new(workerfakes.FakeWorker)
		someWorker1.NameReturns("worker1")
		someWorker2 = new(workerfakes.FakeWorker)
		someWorker2.NameReturns("worker2")
		someWorker3 = new(workerfakes.FakeWorker)
		someWorker3.NameReturns("worker3")

		spec = ContainerSpec{
			ImageSpec: ImageSpec{ResourceType: "some-type"},

			TeamID: 4567,

			Inputs: []InputSource{},
		}
	})

	Context("when there are multiple workers", func() {
		BeforeEach(func() {
			workers = []Worker{someWorker1, someWorker2, someWorker3}

			someWorker1.BuildContainersReturns(30)
			someWorker2.BuildContainersReturns(20)
			someWorker3.BuildContainersReturns(10)
		})

		It("picks the one with least amount of containers", func() {
			Consistently(func() Worker {
				chosenWorker, chooseErr = strategy.Choose(
					logger,
					workers,
					spec,
				)
				Expect(chooseErr).ToNot(HaveOccurred())
				return chosenWorker
			}).Should(Equal(someWorker3))
		})

		Context("when there is more than one worker with the same number of build containers", func() {
			BeforeEach(func() {
				workers = []Worker{someWorker1, someWorker2, someWorker3}
				someWorker1.BuildContainersReturns(10)
				someWorker2.BuildContainersReturns(20)
				someWorker3.BuildContainersReturns(10)

				fakeInput1 := new(workerfakes.FakeInputSource)
				fakeInput1AS := new(workerfakes.FakeArtifactSource)
				fakeInput1AS.ExistsOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
					switch worker {
					case someWorker3:
						return new(workerfakes.FakeVolume), true, nil
					default:
						return nil, false, nil
					}
				}
				fakeInput1.SourceReturns(fakeInput1AS)

				spec = ContainerSpec{
					ImageSpec: ImageSpec{ResourceType: "some-type"},

					TeamID: 4567,

					Inputs: []InputSource{
						fakeInput1,
					},
				}
			})
			It("picks the one with the most volumes", func() {
				Consistently(func() Worker {
					cWorker, cErr := strategy.Choose(
						logger,
						workers,
						spec,
					)
					Expect(cErr).ToNot(HaveOccurred())
					return cWorker
				}).Should(Equal(someWorker3))

			})
		})

	})
})
