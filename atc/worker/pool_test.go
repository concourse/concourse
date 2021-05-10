package worker_test

import (
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"code.cloudfoundry.org/lager/lagertest"
	"context"
	"errors"

	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	. "github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		logger       *lagertest.TestLogger
		fakeProvider *workerfakes.FakeWorkerProvider

		pool Pool
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeProvider = new(workerfakes.FakeWorkerProvider)

		pool = NewPool(fakeProvider)
	})

	Describe("FindContainer", func() {
		var (
			foundContainer Container
			found          bool
			findErr        error
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = pool.FindContainer(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the container", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeContainer *workerfakes.FakeContainer

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForContainerReturns(fakeWorker, true, nil)

				fakeContainer = new(workerfakes.FakeContainer)
				fakeWorker.FindContainerByHandleReturns(fakeContainer, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(foundContainer).To(Equal(fakeContainer))
			})
		})
	})

	Describe("FindVolume", func() {
		var (
			foundVolume Volume
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundVolume, found, findErr = pool.FindVolume(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the volume", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForVolumeReturns(fakeWorker, true, nil)

				fakeVolume = new(workerfakes.FakeVolume)
				fakeWorker.LookupVolumeReturns(fakeVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the volume", func() {
				Expect(foundVolume).To(Equal(fakeVolume))
			})
		})
	})

	Describe("CreateVolume", func() {
		var (
			fakeWorker *workerfakes.FakeWorker
			volumeSpec VolumeSpec
			workerSpec WorkerSpec
			volumeType db.VolumeType
			err        error
		)

		BeforeEach(func() {
			volumeSpec = VolumeSpec{
				Strategy: baggageclaim.EmptyStrategy{},
			}

			workerSpec = WorkerSpec{
				TeamID: 1,
			}

			volumeType = db.VolumeTypeArtifact
		})

		JustBeforeEach(func() {
			_, err = pool.CreateVolume(logger, volumeSpec, workerSpec, volumeType)
		})

		Context("when no workers can be found", func() {
			BeforeEach(func() {
				fakeProvider.RunningWorkersReturns(nil, nil)
			})

			It("returns an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when the worker can be found", func() {
			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.RunningWorkersReturns([]Worker{fakeWorker}, nil)
				fakeWorker.SatisfiesReturns(true)
			})

			It("creates the volume on the worker", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(fakeWorker.CreateVolumeCallCount()).To(Equal(1))
				l, spec, id, t := fakeWorker.CreateVolumeArgsForCall(0)
				Expect(l).To(Equal(logger))
				Expect(spec).To(Equal(volumeSpec))
				Expect(id).To(Equal(1))
				Expect(t).To(Equal(volumeType))
			})
		})
	})

	Describe("SelectWorker", func() {
		var (
			fakeOwner     *dbfakes.FakeContainerOwner
			containerSpec ContainerSpec
			workerSpec    WorkerSpec

			fakeStrategy  *workerfakes.FakeContainerPlacementStrategy
			fakeCallbacks *workerfakes.FakePoolCallbacks

			workerFakes []*workerfakes.FakeWorker
			workers     []Worker

			selectedWorker Client
			selectCtx      context.Context
			selectErr      error
		)

		updateWorkersFromFakes := func() {
			workers = make([]Worker, len(workerFakes))
			for i, fake := range workerFakes {
				workers[i] = Worker(fake)
			}
		}

		BeforeEach(func() {
			fakeOwner = new(dbfakes.FakeContainerOwner)

			containerSpec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},
				TeamID:    4567,
			}

			workerSpec = WorkerSpec{
				ResourceType: "some-type",
				TeamID:       4567,
				Tags:         atc.Tags{"some-tag"},
			}

			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)
			fakeCallbacks = new(workerfakes.FakePoolCallbacks)

			workerFakes = []*workerfakes.FakeWorker{
				new(workerfakes.FakeWorker),
				new(workerfakes.FakeWorker),
				new(workerfakes.FakeWorker),
			}
			for i, worker := range workerFakes {
				worker.NameReturns(fmt.Sprintf("worker-%d", i))
			}

			updateWorkersFromFakes()

			fakeStrategy.OrderCalls(func(_ lager.Logger, workers []Worker, _ ContainerSpec) ([]Worker, error) {
				return append([]Worker(nil), workers...), nil
			})
			fakeStrategy.ApproveReturns(nil)

			fmt.Fprintln(GinkgoWriter, "init-complete")
		})

		Context("when it should return immediately", func() {
			JustBeforeEach(func(done Done) {
				selectCtx = lagerctx.NewContext(context.Background(), logger)

				selectedWorker, _, selectErr = pool.SelectWorker(
					selectCtx,
					fakeOwner,
					containerSpec,
					workerSpec,
					fakeStrategy,
					fakeCallbacks,
				)

				close(done)
			})

			Context("when getting the workers fails", func() {
				var disaster error

				BeforeEach(func() {
					disaster = errors.New("nope")
					fakeProvider.RunningWorkersReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(selectErr).To(Equal(disaster))
				})
			})

			Context("when workers are found with the container", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns(workers, nil)
					fakeProvider.FindWorkersForContainerByOwnerReturns(workers, nil)
				})

				Context("when getting workers with container fails", func() {
					var disaster error

					BeforeEach(func() {
						disaster = errors.New("nuh-uh")
						workerFakes[0].SatisfiesReturns(true)

						fakeProvider.FindWorkersForContainerByOwnerReturns(nil, disaster)
					})

					It("returns the error", func() {
						Expect(selectErr).To(Equal(disaster))
					})
				})

				Context("when one of the workers satisfy the spec", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(true)
						workerFakes[1].SatisfiesReturns(false)
						workerFakes[2].SatisfiesReturns(false)
					})

					It("succeeds and returns the compatible worker with the container", func() {
						Expect(fakeStrategy.OrderCallCount()).To(Equal(0))
						Expect(fakeStrategy.ApproveCallCount()).To(Equal(0))

						Expect(selectErr).NotTo(HaveOccurred())
						Expect(selectedWorker.Name()).To(Equal(workers[0].Name()))
					})
				})

				Context("when multiple workers satisfy the spec", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(true)
						workerFakes[1].SatisfiesReturns(true)
						workerFakes[2].SatisfiesReturns(false)
					})

					It("succeeds and returns the first compatible worker with the container", func() {
						Expect(fakeStrategy.OrderCallCount()).To(Equal(0))
						Expect(fakeStrategy.ApproveCallCount()).To(Equal(0))

						Expect(selectErr).NotTo(HaveOccurred())
						Expect(selectedWorker.Name()).To(Equal(workers[0].Name()))
					})
				})

				Context("when the worker that has the container does not satisfy the spec", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(false)
						workerFakes[1].SatisfiesReturns(true)
						workerFakes[2].SatisfiesReturns(false)

						fakeProvider.FindWorkersForContainerByOwnerReturns([]Worker{workers[2]}, nil)
					})

					It("chooses a satisfying worker", func() {
						Expect(fakeStrategy.OrderCallCount()).To(Equal(1))
						Expect(fakeStrategy.ApproveCallCount()).To(Equal(1))

						_, pickedWorker, _ := fakeStrategy.ApproveArgsForCall(0)
						Expect(pickedWorker.Name()).To(Equal(workers[1].Name()))

						Expect(selectErr).NotTo(HaveOccurred())
						Expect(selectedWorker.Name()).To(Equal(workers[1].Name()))
					})
				})
			})

			Context("when no worker is found with the container", func() {
				BeforeEach(func() {
					fakeProvider.FindWorkersForContainerByOwnerReturns(nil, nil)
				})

				Context("with multiple workers", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(true)
						workerFakes[1].SatisfiesReturns(true)
						workerFakes[2].SatisfiesReturns(false)

						fakeProvider.RunningWorkersReturns(workers, nil)
					})

					It("checks that the workers satisfy the given worker spec", func() {
						Expect(workerFakes[0].SatisfiesCallCount()).To(Equal(1))
						_, actualSpec := workerFakes[0].SatisfiesArgsForCall(0)
						Expect(actualSpec).To(Equal(workerSpec))

						Expect(workerFakes[1].SatisfiesCallCount()).To(Equal(1))
						_, actualSpec = workerFakes[1].SatisfiesArgsForCall(0)
						Expect(actualSpec).To(Equal(workerSpec))

						Expect(workerFakes[2].SatisfiesCallCount()).To(Equal(1))
						_, actualSpec = workerFakes[2].SatisfiesArgsForCall(0)
						Expect(actualSpec).To(Equal(workerSpec))
					})

					It("returns all workers satisfying the spec", func() {
						_, satisfyingWorkers, _ := fakeStrategy.OrderArgsForCall(0)
						Expect(satisfyingWorkers).To(ConsistOf(workers[0], workers[1]))
					})
				})

				Context("when team workers and general workers satisfy the spec", func() {
					BeforeEach(func() {
						extraFake := new(workerfakes.FakeWorker)
						extraFake.NameReturns("extra-fake-1")

						workerFakes = append(workerFakes, extraFake)
						updateWorkersFromFakes()

						workerFakes[0].SatisfiesReturns(true)
						workerFakes[0].IsOwnedByTeamReturns(false)

						workerFakes[1].SatisfiesReturns(true)
						workerFakes[1].IsOwnedByTeamReturns(true)

						workerFakes[2].SatisfiesReturns(true)
						workerFakes[2].IsOwnedByTeamReturns(true)

						workerFakes[3].SatisfiesReturns(false)

						fakeProvider.RunningWorkersReturns(workers, nil)
					})

					It("returns only the team workers that satisfy the spec", func() {
						_, satisfyingWorkers, _ := fakeStrategy.OrderArgsForCall(0)
						Expect(satisfyingWorkers).To(ConsistOf(workerFakes[1], workerFakes[2]))
					})
				})

				Context("when only general workers satisfy the spec", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(false)

						workerFakes[1].SatisfiesReturns(true)
						workerFakes[1].IsOwnedByTeamReturns(false)

						workerFakes[2].SatisfiesReturns(false)

						fakeProvider.RunningWorkersReturns(workers, nil)
					})

					It("returns the general workers that satisfy the spec", func() {
						_, satisfyingWorkers, _ := fakeStrategy.OrderArgsForCall(0)
						Expect(satisfyingWorkers).To(ConsistOf(workerFakes[1]))
					})
				})

				Context("with compatible workers available", func() {
					BeforeEach(func() {
						workerFakes[0].SatisfiesReturns(true)
						workerFakes[1].SatisfiesReturns(true)
						workerFakes[2].SatisfiesReturns(true)

						fakeProvider.RunningWorkersReturns(workers, nil)
					})

					Context("when strategy errors", func() {
						var strategyError error

						BeforeEach(func() {
							strategyError = errors.New("strategical explosion")
							fakeStrategy.OrderReturns(nil, strategyError)
						})

						It("returns an error", func() {
							Expect(selectErr).To(Equal(strategyError))
						})
					})

					Context("when strategy returns a worker", func() {
						BeforeEach(func() {
							fakeStrategy.OrderReturns([]Worker{workers[0]}, nil)
						})

						It("chooses a worker", func() {
							Expect(selectErr).ToNot(HaveOccurred())
							Expect(fakeStrategy.OrderCallCount()).To(Equal(1))
							Expect(selectedWorker.Name()).To(Equal(workers[0].Name()))
						})
					})

					Context("when strategy returns multiple workers", func() {
						BeforeEach(func() {
							fakeStrategy.OrderCalls(func(_ lager.Logger, workers []Worker, _ ContainerSpec) ([]Worker, error) {
								candidates := append([]Worker(nil), workers...)

								// Reverse list to make sure it is properly selecting first worker in list
								for i, j := 0, len(candidates)-1; i < j; i, j = i+1, j-1 {
									candidates[i], candidates[j] = candidates[j], candidates[i]
								}

								return candidates, nil
							})
						})

						It("chooses first worker", func() {
							Expect(fakeStrategy.OrderCallCount()).To(Equal(1))

							_, pickedWorker, _ := fakeStrategy.ApproveArgsForCall(0)
							Expect(pickedWorker.Name()).To(Equal(workers[2].Name()))

							Expect(selectErr).ToNot(HaveOccurred())
							Expect(selectedWorker.Name()).To(Equal(workers[2].Name()))
						})
					})

					Context("when picking the first worker errors", func() {
						BeforeEach(func() {
							fakeStrategy.ApproveReturnsOnCall(0, errors.New("cannot-pick-for-arbitrary-reason"))
						})

						It("succeeds and picks the next worker", func() {
							Expect(fakeStrategy.ApproveCallCount()).To(Equal(2))

							_, pickedWorkerA, _ := fakeStrategy.ApproveArgsForCall(0)
							Expect(pickedWorkerA.Name()).To(Equal(workers[0].Name()))

							_, pickedWorkerB, _ := fakeStrategy.ApproveArgsForCall(1)
							Expect(pickedWorkerB.Name()).To(Equal(workers[1].Name()))

							Expect(selectErr).NotTo(HaveOccurred())
							Expect(selectedWorker.Name()).To(Equal(workers[1].Name()))
						})
					})
				})
			})
		})

		Context("when it should poll for workers", func() {
			JustBeforeEach(func(done Done) {
				selectCtx = lagerctx.NewContext(context.Background(), logger)

				var cancel context.CancelFunc
				selectCtx, cancel = context.WithTimeout(selectCtx, WorkerPollingInterval*3/2)
				defer cancel()

				selectedWorker, _, selectErr = pool.SelectWorker(
					selectCtx,
					fakeOwner,
					containerSpec,
					workerSpec,
					fakeStrategy,
					fakeCallbacks,
				)

				close(done)
			}, (WorkerPollingInterval * 2).Seconds())

			Context("with no workers", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{}, nil)
				})

				It("times out and returns an error", func() {
					Expect(selectErr).To(Equal(selectCtx.Err()))
					Expect(fakeProvider.RunningWorkersCallCount()).To(Equal(2))
				})
			})

			Context("with no compatible workers available", func() {
				BeforeEach(func() {
					workerFakes = workerFakes[:1]
					updateWorkersFromFakes()

					workerFakes[0].SatisfiesReturns(false)
					fakeProvider.RunningWorkersReturns(workers, nil)
				})

				It("times out and returns an error", func() {
					Expect(selectErr).To(Equal(selectCtx.Err()))
					Expect(fakeProvider.RunningWorkersCallCount()).To(Equal(2))
					Expect(workerFakes[0].SatisfiesCallCount()).To(Equal(2))
				})
			})
		})
	})

	Describe("FindWorkersForResourceCache", func() {
		var (
			workerSpec WorkerSpec

			chosenWorkers []Worker
			chooseErr     error

			incompatibleWorker *workerfakes.FakeWorker
			compatibleWorker   *workerfakes.FakeWorker
		)

		BeforeEach(func() {
			workerSpec = WorkerSpec{
				ResourceType: "some-type",
				TeamID:       4567,
				Tags:         atc.Tags{"some-tag"},
			}

			incompatibleWorker = new(workerfakes.FakeWorker)
			incompatibleWorker.SatisfiesReturns(false)

			compatibleWorker = new(workerfakes.FakeWorker)
			compatibleWorker.SatisfiesReturns(true)
		})

		JustBeforeEach(func() {
			chosenWorkers, chooseErr = pool.FindWorkersForResourceCache(
				logger,
				4567,
				1234,
				workerSpec,
			)
		})

		Context("when workers are found with the resource cache", func() {
			var (
				workerA *workerfakes.FakeWorker
				workerB *workerfakes.FakeWorker
				workerC *workerfakes.FakeWorker
			)

			BeforeEach(func() {
				workerA = new(workerfakes.FakeWorker)
				workerA.NameReturns("workerA")
				workerB = new(workerfakes.FakeWorker)
				workerB.NameReturns("workerB")
				workerC = new(workerfakes.FakeWorker)
				workerC.NameReturns("workerC")

				fakeProvider.FindWorkersForResourceCacheReturns([]Worker{workerA, workerB, workerC}, nil)
			})

			Context("when one of the workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the compatible worker with the resource cache", func() {
					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(len(chosenWorkers)).To(Equal(1))
					Expect(chosenWorkers[0].Name()).To(Equal(workerA.Name()))
				})
			})

			Context("when multiple workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the first compatible worker with the container", func() {
					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(len(chosenWorkers)).To(Equal(2))
					Expect(chosenWorkers[0].Name()).To(Equal(workerA.Name()))
					Expect(chosenWorkers[1].Name()).To(Equal(workerB.Name()))
				})
			})

			Context("when the worker that has the resource cache does not satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)

					fakeProvider.FindWorkersForResourceCacheReturns([]Worker{workerC}, nil)
				})

				It("returns empty worker list", func() {
					Expect(chooseErr).ToNot(HaveOccurred())
					Expect(chosenWorkers).To(BeEmpty())
				})
			})
		})
	})
})
