package worker_test

import (
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
		pool         Pool
		fakeProvider *workerfakes.FakeWorkerProvider
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
			spec       ContainerSpec
			workerSpec WorkerSpec
			fakeOwner  *dbfakes.FakeContainerOwner

			chosenWorker Client
			chooseErr    error

			incompatibleWorker *workerfakes.FakeWorker
			compatibleWorker   *workerfakes.FakeWorker
			fakeStrategy       *workerfakes.FakeContainerPlacementStrategy
		)

		BeforeEach(func() {
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

			fakeOwner = new(dbfakes.FakeContainerOwner)

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},
				TeamID:    4567,
			}

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
			chosenWorker, chooseErr = pool.SelectWorker(
				context.Background(),
				fakeOwner,
				spec,
				workerSpec,
				fakeStrategy,
			)
		})

		Context("when workers are found with the container", func() {
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

				fakeProvider.FindWorkersForContainerByOwnerReturns([]Worker{workerA, workerB, workerC}, nil)
				fakeProvider.RunningWorkersReturns([]Worker{workerA, workerB, workerC}, nil)
				fakeStrategy.ChooseReturns(workerA, nil)
			})

			Context("when one of the workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the compatible worker with the container", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(0))

					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(chosenWorker.Name()).To(Equal(workerA.Name()))
				})
			})

			Context("when multiple workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)
				})

				It("succeeds and returns the first compatible worker with the container", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(0))

					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(chosenWorker.Name()).To(Equal(workerA.Name()))
				})
			})

			Context("when the worker that has the container does not satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)

					fakeProvider.FindWorkersForContainerByOwnerReturns([]Worker{workerC}, nil)
				})

				It("chooses a satisfying worker", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(1))

					Expect(chooseErr).NotTo(HaveOccurred())
					Expect(chosenWorker.Name()).ToNot(Equal(workerC.Name()))
				})
			})
		})

		Context("when no worker is found with the container", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkersForContainerByOwnerReturns(nil, nil)
			})

			Context("with multiple workers", func() {
				var (
					workerA *workerfakes.FakeWorker
					workerB *workerfakes.FakeWorker
					workerC *workerfakes.FakeWorker
				)

				BeforeEach(func() {
					workerA = new(workerfakes.FakeWorker)
					workerB = new(workerfakes.FakeWorker)
					workerC = new(workerfakes.FakeWorker)
					workerA.NameReturns("workerA")

					workerA.SatisfiesReturns(true)
					workerB.SatisfiesReturns(true)
					workerC.SatisfiesReturns(false)

					fakeProvider.RunningWorkersReturns([]Worker{workerA, workerB, workerC}, nil)
					fakeStrategy.ChooseReturns(workerA, nil)
				})

				It("checks that the workers satisfy the given worker spec", func() {
					Expect(workerA.SatisfiesCallCount()).To(Equal(1))
					_, actualSpec := workerA.SatisfiesArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerB.SatisfiesCallCount()).To(Equal(1))
					_, actualSpec = workerB.SatisfiesArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerC.SatisfiesCallCount()).To(Equal(1))
					_, actualSpec = workerC.SatisfiesArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))
				})

				It("returns all workers satisfying the spec", func() {
					_, satisfyingWorkers, _ := fakeStrategy.ChooseArgsForCall(0)
					Expect(satisfyingWorkers).To(ConsistOf(workerA, workerB))
				})
			})

			Context("when team workers and general workers satisfy the spec", func() {
				var (
					teamWorker1   *workerfakes.FakeWorker
					teamWorker2   *workerfakes.FakeWorker
					teamWorker3   *workerfakes.FakeWorker
					generalWorker *workerfakes.FakeWorker
				)

				BeforeEach(func() {
					teamWorker1 = new(workerfakes.FakeWorker)
					teamWorker1.SatisfiesReturns(true)
					teamWorker1.IsOwnedByTeamReturns(true)
					teamWorker2 = new(workerfakes.FakeWorker)
					teamWorker2.SatisfiesReturns(true)
					teamWorker2.IsOwnedByTeamReturns(true)
					teamWorker3 = new(workerfakes.FakeWorker)
					teamWorker3.SatisfiesReturns(false)
					generalWorker = new(workerfakes.FakeWorker)
					generalWorker.SatisfiesReturns(true)
					generalWorker.IsOwnedByTeamReturns(false)
					fakeProvider.RunningWorkersReturns([]Worker{generalWorker, teamWorker1, teamWorker2, teamWorker3}, nil)
					fakeStrategy.ChooseReturns(teamWorker1, nil)
				})

				It("returns only the team workers that satisfy the spec", func() {
					_, satisfyingWorkers, _ := fakeStrategy.ChooseArgsForCall(0)
					Expect(satisfyingWorkers).To(ConsistOf(teamWorker1, teamWorker2))
				})
			})

			Context("when only general workers satisfy the spec", func() {
				var (
					teamWorker     *workerfakes.FakeWorker
					generalWorker1 *workerfakes.FakeWorker
					generalWorker2 *workerfakes.FakeWorker
				)

				BeforeEach(func() {
					teamWorker = new(workerfakes.FakeWorker)
					teamWorker.SatisfiesReturns(false)
					generalWorker1 = new(workerfakes.FakeWorker)
					generalWorker1.SatisfiesReturns(true)
					generalWorker1.IsOwnedByTeamReturns(false)
					generalWorker2 = new(workerfakes.FakeWorker)
					generalWorker2.SatisfiesReturns(false)
					fakeProvider.RunningWorkersReturns([]Worker{generalWorker1, generalWorker2, teamWorker}, nil)
					fakeStrategy.ChooseReturns(generalWorker1, nil)
				})

				It("returns the general workers that satisfy the spec", func() {
					_, satisfyingWorkers, _ := fakeStrategy.ChooseArgsForCall(0)
					Expect(satisfyingWorkers).To(ConsistOf(generalWorker1))
				})
			})

			Context("with no workers", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{}, nil)
				})

				It("returns ErrNoWorkers", func() {
					Expect(chooseErr).To(Equal(ErrNoWorkers))
				})
			})

			Context("when getting the workers fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(chooseErr).To(Equal(disaster))
				})
			})

			Context("with no compatible workers available", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{incompatibleWorker}, nil)
				})

				It("returns NoCompatibleWorkersError", func() {
					Expect(chooseErr).To(Equal(NoCompatibleWorkersError{
						Spec: workerSpec,
					}))
				})
			})

			Context("with compatible workers available", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{
						incompatibleWorker,
						compatibleWorker,
					}, nil)
				})

				Context("when strategy returns a worker", func() {
					BeforeEach(func() {
						fakeStrategy.ChooseReturns(compatibleWorker, nil)
					})

					It("chooses a worker", func() {
						Expect(chooseErr).ToNot(HaveOccurred())
						Expect(fakeStrategy.ChooseCallCount()).To(Equal(1))
						Expect(chosenWorker.Name()).To(Equal(compatibleWorker.Name()))
					})
				})

				Context("when strategy errors", func() {
					var (
						strategyError error
					)

					BeforeEach(func() {
						strategyError = errors.New("strategical explosion")
						fakeStrategy.ChooseReturns(nil, strategyError)
					})

					It("returns an error", func() {
						Expect(chooseErr).To(Equal(strategyError))
					})
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

				It("returns NoCompatibleWorkersError", func() {
					Expect(chooseErr).To(Equal(NoCompatibleWorkersError{
						Spec: workerSpec,
					}))
				})
			})
		})

		Context("when no worker is found with the resource cache", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkersForResourceCacheReturns(nil, nil)
			})

			It("returns NoCompatibleWorkersError", func() {
				Expect(chooseErr).To(Equal(ErrNoWorkers))
			})
		})
	})
})
