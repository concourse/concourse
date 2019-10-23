package worker_test

import (
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"context"
	"errors"

	"github.com/concourse/concourse/atc"
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

	Describe("FindOrChooseWorkerForContainer", func() {
		var (
			spec          ContainerSpec
			workerSpec    WorkerSpec
			resourceTypes atc.VersionedResourceTypes
			fakeOwner     *dbfakes.FakeContainerOwner

			chosenWorker Worker
			chooseErr    error

			incompatibleWorker *workerfakes.FakeWorker
			compatibleWorker   *workerfakes.FakeWorker
			fakeStrategy       *workerfakes.FakeContainerPlacementStrategy
		)

		BeforeEach(func() {
			fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

			fakeOwner = new(dbfakes.FakeContainerOwner)

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

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-b",
						Type:   "custom-type-a",
						Source: atc.Source{"some": "super-secret-source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}

			workerSpec = WorkerSpec{
				ResourceType:  "some-type",
				TeamID:        4567,
				Tags:          atc.Tags{"some-tag"},
				ResourceTypes: resourceTypes,
			}

			incompatibleWorker = new(workerfakes.FakeWorker)
			incompatibleWorker.SatisfiesReturns(false)

			compatibleWorker = new(workerfakes.FakeWorker)
			compatibleWorker.SatisfiesReturns(true)
		})

		JustBeforeEach(func() {
			chosenWorker, chooseErr = pool.FindOrChooseWorkerForContainer(
				context.TODO(),
				logger,
				fakeOwner,
				spec,
				workerSpec,
				fakeStrategy,
			)
		})

		Context("selects a worker in serial", func() {
			var (
				workerA *workerfakes.FakeWorker
			)

			BeforeEach(func() {
				workerA = new(workerfakes.FakeWorker)
				workerA.NameReturns("workerA")
				workerA.SatisfiesReturns(true)

				fakeProvider.FindWorkersForContainerByOwnerReturns([]Worker{workerA}, nil)
				fakeProvider.RunningWorkersReturns([]Worker{workerA}, nil)
				fakeStrategy.ChooseReturns(workerA, nil)
			})

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
				workerA.SatisfiesReturns(true)
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

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(false)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("returns a NoCompatibleWorkersError", func() {
					Expect(chooseErr).To(Equal(NoCompatibleWorkersError{
						Spec: workerSpec,
					}))
				})
			})

			Context("when the worker that have the container does not satisfy the spec", func() {
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

				Context("when no workers satisfy the spec", func() {
					BeforeEach(func() {
						workerA.SatisfiesReturns(false)
						workerB.SatisfiesReturns(false)
						workerC.SatisfiesReturns(false)
					})

					It("returns a NoCompatibleWorkersError", func() {
						Expect(chooseErr).To(Equal(NoCompatibleWorkersError{
							Spec: workerSpec,
						}))
					})
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

			Context("with no workers available", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{}, nil)
				})

				It("returns ErrNoWorkers", func() {
					Expect(chooseErr).To(Equal(ErrNoWorkers))
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

})
