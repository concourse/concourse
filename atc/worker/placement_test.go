package worker_test

import (
	"errors"
	"fmt"
	"strings"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

//counterfeiter:generate . ContainerPlacementStrategy

var _ = Describe("ContainerPlacementStrategy", func() {
	var (
		logger      *lagertest.TestLogger
		strategy    ContainerPlacementStrategy
		strategyErr error

		containerSpec ContainerSpec
		workerFakes   []*workerfakes.FakeWorker
		workers       []Worker

		orderedWorkers []Worker
		orderErr       error

		pickedWorker Worker
		pickErr      error
	)

	printWorkers := func(format string, workers []Worker) {
		names := make([]string, len(workers))
		for i, worker := range workers {
			names[i] = worker.Name()
		}

		fmt.Fprintln(GinkgoWriter, fmt.Sprintf(format, strings.Join(names, ", ")))
	}

	updateWorkersFromFakes := func() {
		workers = make([]Worker, len(workerFakes))
		for i, fake := range workerFakes {
			workers[i] = Worker(fake)
		}
	}

	makeFakeInput := func(workersWithInput ...Worker) InputSource {
		artifactSource := new(workerfakes.FakeArtifactSource)
		artifactSource.ExistsOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
			for _, hasInput := range workersWithInput {
				if worker == hasInput {
					return new(workerfakes.FakeVolume), true, nil
				}
			}

			return nil, false, nil
		}

		input := new(workerfakes.FakeInputSource)
		input.SourceReturns(artifactSource)

		return input
	}

	order := func(assertErr bool) []Worker {
		printWorkers("inital workers: %s", workers)
		orderedWorkers, orderErr = strategy.Order(logger, append([]Worker(nil), workers...), containerSpec)

		if assertErr {
			Expect(orderErr).ToNot(HaveOccurred())
		}

		printWorkers("ordered workers: %s", orderedWorkers)
		return orderedWorkers
	}

	pickAndRelease := func() Worker {
		pickedWorker = nil

		for _, worker := range orderedWorkers {
			pickErr = strategy.Pick(logger, worker, containerSpec)

			if pickErr == nil {
				pickedWorker = worker
				break
			}
		}

		if pickedWorker != nil {
			fmt.Fprintln(GinkgoWriter, fmt.Sprintf("picked worker: %s", pickedWorker.Name()))
			strategy.Release(logger, pickedWorker, containerSpec)
		}

		return pickedWorker
	}

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("placement-tests")

		containerSpec = ContainerSpec{
			ImageSpec: ImageSpec{ResourceType: "some-type"},
			TeamID:    4567,
			Inputs:    []InputSource{},
		}

		workerFakes = []*workerfakes.FakeWorker{
			new(workerfakes.FakeWorker),
			new(workerfakes.FakeWorker),
			new(workerfakes.FakeWorker),
		}
		for i, worker := range workerFakes {
			worker.NameReturns(fmt.Sprintf("worker-%d", i))
		}

		updateWorkersFromFakes()
		fmt.Fprintln(GinkgoWriter, "init-complete")
	})

	Describe("No strategy", func() {

		BeforeEach(func() {
			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{},
			})
			Expect(strategyErr).ToNot(HaveOccurred())

		})

		Describe("strategy.Order", func() {
			It("orders candidates randomly", func() {
				Consistently(func() []Worker {
					return order(true)
				}).Should(SatisfyAny(
					Equal([]Worker{workers[0], workers[1], workers[2]}),
					Equal([]Worker{workers[0], workers[2], workers[1]}),
					Equal([]Worker{workers[1], workers[0], workers[2]}),
					Equal([]Worker{workers[1], workers[2], workers[0]}),
					Equal([]Worker{workers[2], workers[0], workers[1]}),
					Equal([]Worker{workers[2], workers[1], workers[0]}),
				))
			})
		})
	})

	Describe("fewest-build-containers", func() {
		JustBeforeEach(func() {
			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"fewest-build-containers"},
			})
			Expect(strategyErr).ToNot(HaveOccurred())
		})

		Describe("strategy.Order", func() {
			JustBeforeEach(func() {
				order(true)
			})

			Context("with multiple workers", func() {
				BeforeEach(func() {
					workerFakes[0].BuildContainersReturns(20)
					workerFakes[1].BuildContainersReturns(30)
					workerFakes[2].BuildContainersReturns(10)
				})

				It("orders workers by build container count", func() {
					Expect(orderedWorkers).To(Equal([]Worker{workers[2], workers[0], workers[1]}))
				})

				Context("when multiple have the same number of build containers", func() {
					BeforeEach(func() {
						workerFakes[1].BuildContainersReturns(10)
					})

					It("orders workers with same counts randomly", func() {
						Consistently(func() []Worker {
							return order(true)
						}).Should(SatisfyAny(
							Equal([]Worker{workers[1], workers[2], workers[0]}),
							Equal([]Worker{workers[2], workers[1], workers[0]}),
						))
					})
				})
			})
		})
	})

	Describe("volume-locality", func() {
		JustBeforeEach(func() {
			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"volume-locality"},
			})
			Expect(strategyErr).ToNot(HaveOccurred())
		})

		Describe("strategy.Order", func() {
			JustBeforeEach(func() {
				order(true)
			})

			BeforeEach(func() {
				extraFake := new(workerfakes.FakeWorker)
				extraFake.NameReturns("extra-fake-1")

				workerFakes = append(workerFakes, extraFake)
				updateWorkersFromFakes()
			})

			Context("with multiple workers", func() {
				BeforeEach(func() {
					fakeInput1 := makeFakeInput(workers[0], workers[1])
					fakeInput2 := makeFakeInput(workers[0], workers[3])

					containerSpec.Inputs = []InputSource{
						fakeInput1,
						fakeInput2,
					}
				})

				It("orders workers by existing volume count", func() {
					Consistently(func() []Worker {
						return order(true)
					}).Should(SatisfyAny(
						Equal([]Worker{workers[0], workers[1], workers[3], workers[2]}),
						Equal([]Worker{workers[0], workers[3], workers[1], workers[2]}),
					))
				})

				Context("when multiple have same volume count", func() {
					BeforeEach(func() {
						fakeInput1 := makeFakeInput(workers[0], workers[1])
						fakeInput2 := makeFakeInput(workers[0], workers[1], workers[2])

						containerSpec.Inputs = []InputSource{
							fakeInput1,
							fakeInput2,
						}
					})

					It("orders workers with same count randomly", func() {
						Consistently(func() []Worker {
							return order(true)
						}).Should(SatisfyAny(
							Equal([]Worker{workers[0], workers[1], workers[2], workers[3]}),
							Equal([]Worker{workers[1], workers[0], workers[2], workers[3]}),
						))
					})
				})
			})

			Context("when no worker having any volumes", func() {
				BeforeEach(func() {
					fakeInput1 := makeFakeInput()
					fakeInput2 := makeFakeInput()

					containerSpec.Inputs = []InputSource{
						fakeInput1,
						fakeInput2,
					}

					workerFakes = workerFakes[:2]
					updateWorkersFromFakes()
				})

				It("orders all workers randomly", func() {
					Consistently(func() []Worker {
						return order(true)
					}).Should(SatisfyAny(
						Equal([]Worker{workers[0], workers[1]}),
						Equal([]Worker{workers[1], workers[0]}),
					))
				})
			})
		})
	})

	Describe("limit-active-tasks", func() {
		var limit int
		var shouldError bool

		BeforeEach(func() {
			limit = -1
			shouldError = true
		})

		JustBeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, fmt.Sprintf("limit: %d, should error: %t", limit, shouldError))

			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"limit-active-tasks"},
				MaxActiveTasksPerWorker:    limit,
			})

			if !shouldError {
				Expect(strategyErr).ToNot(HaveOccurred())
			} else {
				Expect(strategyErr).To(HaveOccurred())
			}

			containerSpec.Type = "task"
		})

		Context("when max-tasks-per-worker less than 0", func() {
			It("should fail", func() {
				Expect(strategyErr).To(Equal(errors.New("max-active-tasks-per-worker must be greater or equal than 0")))
				Expect(strategy).To(BeNil())
			})
		})

		Context("when max-tasks-per-worker is configured correctly", func() {
			BeforeEach(func() {
				limit = 0
				shouldError = false
			})

			Describe("strategy.Order", func() {
				JustBeforeEach(func() {
					order(true)
				})

				Context("when only one worker has running tasks", func() {
					BeforeEach(func() {
						workerFakes[1].ActiveTasksReturns(42, nil)
					})

					It("returns that worker last", func() {
						Consistently(func() []Worker {
							return order(true)
						}).Should(SatisfyAny(
							Equal([]Worker{workers[0], workers[2], workers[1]}),
							Equal([]Worker{workers[2], workers[0], workers[1]}),
						))
					})
				})

				Context("with multiple workers", func() {
					BeforeEach(func() {
						workerFakes[0].ActiveTasksReturns(3, nil)
						workerFakes[1].ActiveTasksReturns(1, nil)
						workerFakes[2].ActiveTasksReturns(2, nil)
					})

					It("orders workers by active task count", func() {
						Expect(orderedWorkers).To(Equal([]Worker{workers[1], workers[2], workers[0]}))
					})

					Context("when multiple have the same number of build containers", func() {
						BeforeEach(func() {
							workerFakes[1].ActiveTasksReturns(2, nil)
						})

						It("orders workers with same counts randomly", func() {
							Consistently(func() []Worker {
								return order(true)
							}).Should(SatisfyAny(
								Equal([]Worker{workers[1], workers[2], workers[0]}),
								Equal([]Worker{workers[2], workers[1], workers[0]}),
							))
						})
					})

					Context("when there is an error getting the active task count", func() {
						BeforeEach(func() {
							workerFakes[2].ActiveTasksReturns(0, errors.New("unable-to-get-task-count"))
						})

						It("ignores the failed worker", func() {
							Expect(orderedWorkers).To(Equal([]Worker{workers[1], workers[0]}))
						})
					})

					Context("and a non-zero limit", func() {
						BeforeEach(func() {
							limit = 3
						})

						It("still returns all workers, even those beyond the limit", func() {
							Expect(orderedWorkers).To(Equal([]Worker{workers[1], workers[2], workers[0]}))
						})
					})

					Context("and a non-task step", func() {
						BeforeEach(func() {
							limit = 1
							containerSpec.Type = "check"

							workerFakes = workerFakes[:2]
							updateWorkersFromFakes()
						})

						It("returns workers in a random order", func() {
							Consistently(func() []Worker {
								return order(true)
							}).Should(SatisfyAny(
								Equal([]Worker{workers[0], workers[1]}),
								Equal([]Worker{workers[1], workers[0]}),
							))
						})
					})
				})
			})

			Describe("strategy.Pick and strategy.Release", func() {
				JustBeforeEach(func() {
					pickAndRelease()
				})

				BeforeEach(func() {
					workerFakes[0].IncreaseActiveTasksReturns(4, nil)
					workerFakes[1].IncreaseActiveTasksReturns(2, nil)
					workerFakes[2].IncreaseActiveTasksReturns(3, nil)

					orderedWorkers = workers
				})

				Context("when limit is zero", func() {
					It("is able to pick and release the first worker, regardless of active tasks", func() {
						Expect(pickedWorker).To(Equal(workers[0]))
					})
				})

				Context("when limit is non-zero", func() {
					BeforeEach(func() {
						limit = 2
					})

					It("fails to pick workers with an equal or higher number of tasks", func() {
						Expect(pickedWorker).To(Equal(workers[1]))
					})

					It("increments and decrements active tasks for picked worker", func() {
						Expect(workerFakes[1].IncreaseActiveTasksCallCount()).To(Equal(1))
						Expect(workerFakes[1].DecreaseActiveTasksCallCount()).To(Equal(1))
					})
				})

				Context("when no workers are under the limit", func() {
					BeforeEach(func() {
						limit = 1
					})

					It("fails to pick workers with an equal or higher number of tasks", func() {
						Expect(pickedWorker).To(BeNil())
						Expect(pickErr).To(Equal(ErrTooManyActiveTasks))
					})
				})
			})
		})
	})

	Describe("limit-active-containers", func() {
		var limit int
		var shouldError bool

		BeforeEach(func() {
			limit = -1
			shouldError = true
		})

		JustBeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, fmt.Sprintf("limit: %d, should error: %t", limit, shouldError))

			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy:   []string{"limit-active-containers"},
				MaxActiveContainersPerWorker: limit,
			})

			if !shouldError {
				Expect(strategyErr).ToNot(HaveOccurred())
			} else {
				Expect(strategyErr).To(HaveOccurred())
			}
		})

		Context("when max-active-containers-per-worker less than 0", func() {
			It("should fail", func() {
				Expect(strategyErr).To(Equal(errors.New("max-active-containers-per-worker must be greater or equal than 0")))
				Expect(strategy).To(BeNil())
			})
		})

		Describe("strategy.Order", func() {
			JustBeforeEach(func() {
				order(true)
			})

			BeforeEach(func() {
				workerFakes[0].ActiveContainersReturns(200)
				workerFakes[1].ActiveContainersReturns(20)
				workerFakes[2].ActiveContainersReturns(20000)

				shouldError = false
			})

			Context("when the limit is zero", func() {
				BeforeEach(func() {
					limit = 0
				})

				It("returns all workers in a random order", func() {
					Expect(orderedWorkers).To(ConsistOf([]Worker{workers[0], workers[1], workers[2]}))
				})
			})

			Context("when the limit is non-zero", func() {
				BeforeEach(func() {
					limit = 100
				})

				It("still returns all workers in a random order", func() {
					Expect(orderedWorkers).To(ConsistOf([]Worker{workers[0], workers[1], workers[2]}))
				})
			})
		})

		Describe("strategy.Pick and strategy.Release", func() {
			JustBeforeEach(func() {
				pickAndRelease()
			})

			BeforeEach(func() {
				workerFakes[0].ActiveContainersReturns(200)
				workerFakes[1].ActiveContainersReturns(20)
				workerFakes[2].ActiveContainersReturns(20000)

				orderedWorkers = workers
				shouldError = false
			})

			Context("when limit is zero", func() {
				BeforeEach(func() {
					limit = 0
				})

				It("is able to pick and release the first worker, regardless of active containers", func() {
					Expect(pickedWorker).To(Equal(workers[0]))
				})
			})

			Context("when limit is non-zero", func() {
				BeforeEach(func() {
					limit = 100
				})

				It("fails to pick workers with an equal or higher number of containers", func() {
					Expect(pickedWorker).To(Equal(workers[1]))
				})
			})

			Context("when no workers are under the limit", func() {
				BeforeEach(func() {
					limit = 10
				})

				It("fails to pick workers with an equal or higher number of containers", func() {
					Expect(pickedWorker).To(BeNil())
					Expect(pickErr).To(Equal(ErrTooManyContainers))
				})
			})
		})
	})

	Describe("limit-active-volumes", func() {
		var limit int
		var shouldError bool

		BeforeEach(func() {
			limit = -1
			shouldError = true
		})

		JustBeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, fmt.Sprintf("limit: %d, should error: %t", limit, shouldError))

			strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
				ContainerPlacementStrategy: []string{"limit-active-volumes"},
				MaxActiveVolumesPerWorker:  limit,
			})

			if !shouldError {
				Expect(strategyErr).ToNot(HaveOccurred())
			} else {
				Expect(strategyErr).To(HaveOccurred())
			}
		})

		Context("when max-active-volumes-per-worker less than 0", func() {
			It("should fail", func() {
				Expect(strategyErr).To(Equal(errors.New("max-active-volumes-per-worker must be greater or equal than 0")))
				Expect(strategy).To(BeNil())
			})
		})

		Describe("strategy.Order", func() {
			JustBeforeEach(func() {
				order(true)
			})

			BeforeEach(func() {
				workerFakes[0].ActiveVolumesReturns(200)
				workerFakes[1].ActiveVolumesReturns(20000)
				workerFakes[2].ActiveVolumesReturns(20)

				shouldError = false
			})

			Context("when the limit is zero", func() {
				BeforeEach(func() {
					limit = 0
				})

				It("returns all workers in a random order", func() {
					Expect(orderedWorkers).To(ConsistOf([]Worker{workers[0], workers[1], workers[2]}))
				})
			})

			Context("when the limit is non-zero", func() {
				BeforeEach(func() {
					limit = 100
				})

				It("returns all workers in a random order", func() {
					Expect(orderedWorkers).To(ConsistOf([]Worker{workers[0], workers[1], workers[2]}))
				})
			})
		})

		Describe("strategy.Pick and strategy.Release", func() {
			JustBeforeEach(func() {
				pickAndRelease()
			})

			BeforeEach(func() {
				workerFakes[0].ActiveVolumesReturns(200)
				workerFakes[1].ActiveVolumesReturns(20000)
				workerFakes[2].ActiveVolumesReturns(20)

				orderedWorkers = workers
				shouldError = false
			})

			Context("when limit is zero", func() {
				BeforeEach(func() {
					limit = 0
				})

				It("is able to pick and release the first worker, regardless of active volumes", func() {
					Expect(pickedWorker).To(Equal(workers[0]))
				})
			})

			Context("when limit is non-zero", func() {
				BeforeEach(func() {
					limit = 100
				})

				It("fails to pick workers with an equal or higher number of volumes", func() {
					Expect(pickedWorker).To(Equal(workers[2]))
				})
			})

			Context("when no workers are under the limit", func() {
				BeforeEach(func() {
					limit = 10
				})

				It("fails to pick workers with an equal or higher number of volumes", func() {
					Expect(pickedWorker).To(BeNil())
					Expect(pickErr).To(Equal(ErrTooManyVolumes))
				})
			})
		})
	})

	Describe("Chained placement strategy", func() {
		Describe("strategy.Order", func() {
			Context("fewest-build-containers,volume-locality", func() {
				JustBeforeEach(func() {
					strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
						ContainerPlacementStrategy: []string{"fewest-build-containers", "volume-locality"},
					})
					Expect(strategyErr).ToNot(HaveOccurred())

					order(true)
				})

				BeforeEach(func() {
					workerFakes[0].BuildContainersReturns(30)
					workerFakes[1].BuildContainersReturns(20)
					workerFakes[2].BuildContainersReturns(10)

					fakeInput1 := makeFakeInput(workers[0], workers[1])
					fakeInput2 := makeFakeInput(workers[0], workers[2])

					containerSpec.Inputs = []InputSource{
						fakeInput1,
						fakeInput2,
					}
				})

				It("orders by build containers first", func() {
					Expect(orderedWorkers).To(Equal([]Worker{workers[2], workers[1], workers[0]}))
				})

				Context("when two workers have the same number of build containers", func() {
					BeforeEach(func() {
						workerFakes[0].BuildContainersReturns(10)
					})

					It("breaks ties using volume-locality strategy", func() {
						Expect(orderedWorkers).To(Equal([]Worker{workers[0], workers[2], workers[1]}))
					})
				})

				Context("when two workers have same order", func() {
					BeforeEach(func() {
						workerFakes[1].BuildContainersReturns(10)
					})

					It("orders them randomly", func() {
						Consistently(func() []Worker {
							return order(true)
						}).Should(SatisfyAny(
							Equal([]Worker{workers[1], workers[2], workers[0]}),
							Equal([]Worker{workers[2], workers[1], workers[0]}),
						))
					})
				})
			})

			Context("limit-active-containers,volume-locality", func() {
				JustBeforeEach(func() {
					strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
						ContainerPlacementStrategy:   []string{"limit-active-containers", "volume-locality"},
						MaxActiveContainersPerWorker: 0,
					})
					Expect(strategyErr).ToNot(HaveOccurred())

					order(true)
				})

				BeforeEach(func() {
					workerFakes[0].ActiveContainersReturns(30)
					workerFakes[1].ActiveContainersReturns(20)
					workerFakes[2].ActiveContainersReturns(10)

					fakeInput1 := makeFakeInput(workers[0], workers[1])
					fakeInput2 := makeFakeInput(workers[0], workers[2])

					containerSpec.Inputs = []InputSource{
						fakeInput1,
						fakeInput2,
					}
				})

				It("orders only by volume locality and not active container count", func() {
					Consistently(func() []Worker {
						return order(true)
					}).Should(SatisfyAny(
						Equal([]Worker{workers[0], workers[1], workers[2]}),
						Equal([]Worker{workers[0], workers[2], workers[1]}),
					))
				})
			})
		})

		Describe("strategy.Pick and strategy.Release", func() {
			Context("limit-active-containers,limit-active-tasks", func() {
				JustBeforeEach(func() {
					strategy, strategyErr = NewChainPlacementStrategy(ContainerPlacementStrategyOptions{
						ContainerPlacementStrategy:   []string{"limit-active-containers", "limit-active-tasks"},
						MaxActiveTasksPerWorker:      1,
						MaxActiveContainersPerWorker: 1,
					})

					Expect(strategyErr).ToNot(HaveOccurred())

					containerSpec.Type = "task"
					orderedWorkers = workers[:1]

					pickAndRelease()
				})

				It("calls .Pick and .Release on chained strategies", func() {
					// From "limit-active-containers" strategy
					Expect(workerFakes[0].ActiveContainersCallCount()).To(Equal(1))

					// From "limit-active-tasks" strategy
					Expect(workerFakes[0].IncreaseActiveTasksCallCount()).To(Equal(1))
					Expect(workerFakes[0].DecreaseActiveTasksCallCount()).To(Equal(1))
				})

				Context("when first strategy rejects worker", func() {
					BeforeEach(func() {
						// Causes "limit-active-containers" strategy to fail in .Pick
						workerFakes[0].ActiveContainersReturns(2)
					})

					It("exits early and doesn't call .Pick on later strategies", func() {
						// From "limit-active-containers" strategy
						Expect(workerFakes[0].ActiveContainersCallCount()).To(Equal(1))

						// From "limit-active-tasks" strategy
						Expect(workerFakes[0].IncreaseActiveTasksCallCount()).To(Equal(0))
						Expect(workerFakes[0].DecreaseActiveTasksCallCount()).To(Equal(0))
					})
				})
			})
		})
	})
})
