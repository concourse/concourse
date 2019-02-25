package worker_test

import (
	"context"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
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
		fakeStrategy *workerfakes.FakeContainerPlacementStrategy
		pool         Client
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeProvider = new(workerfakes.FakeWorkerProvider)
		fakeStrategy = new(workerfakes.FakeContainerPlacementStrategy)

		pool = NewPool(fakeProvider, fakeStrategy)
	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingErr    error
			satisfyingWorker Worker
			resourceTypes    creds.VersionedResourceTypes
		)

		BeforeEach(func() {
			variables := template.StaticVariables{
				"secret-source": "super-secret-source",
			}

			resourceTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-resource-type",
						Type:   "some-underlying-type",
						Source: atc.Source{"some": "((secret-source))"},
					},
					Version: atc.Version{"some": "version"},
				},
			})

			spec = WorkerSpec{
				Platform:      "some-platform",
				Tags:          []string{"step", "tags"},
				ResourceTypes: resourceTypes,
			}
		})

		JustBeforeEach(func() {
			satisfyingWorker, satisfyingErr = pool.Satisfying(logger, spec)
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

				workerA.SatisfyingReturns(workerA, nil)
				workerB.SatisfyingReturns(workerB, nil)
				workerC.SatisfyingReturns(nil, errors.New("nope"))

				fakeProvider.RunningWorkersReturns([]Worker{workerA, workerB, workerC}, nil)
			})

			It("succeeds", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
			})

			It("checks that the workers satisfy the given spec", func() {
				Expect(workerA.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec := workerA.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))

				Expect(workerB.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec = workerB.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))

				Expect(workerC.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec = workerC.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
			})

			It("returns a random worker satisfying the spec", func() {
				chosenCount := map[Worker]int{workerA: 0, workerB: 0, workerC: 0}
				for i := 0; i < 100; i++ {
					satisfyingWorker, satisfyingErr = pool.Satisfying(logger, spec)
					Expect(satisfyingErr).NotTo(HaveOccurred())
					chosenCount[satisfyingWorker]++
				}
				Expect(chosenCount[workerA]).To(BeNumerically("~", chosenCount[workerB], 50))
				Expect(chosenCount[workerC]).To(BeZero())
			})

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfyingReturns(nil, errors.New("nope"))
					workerB.SatisfyingReturns(nil, errors.New("nope"))
					workerC.SatisfyingReturns(nil, errors.New("nope"))
				})

				It("returns a NoCompatibleWorkersError", func() {
					Expect(satisfyingErr).To(Equal(NoCompatibleWorkersError{
						Spec: spec,
					}))
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.RunningWorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				Expect(satisfyingErr).To(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.RunningWorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(satisfyingErr).To(Equal(disaster))
			})
		})
	})

	Describe("FindContainerByHandle", func() {
		var (
			foundContainer Container
			found          bool
			findErr        error
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = pool.FindContainerByHandle(
				logger,
				4567,
				"some-handle",
			)
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

			It("finds on the particular worker", func() {
				Expect(fakeWorker.FindContainerByHandleCallCount()).To(Equal(1))

				_, actualTeamID, actualHandle := fakeProvider.FindWorkerForContainerArgsForCall(0)
				Expect(actualTeamID).To(Equal(4567))
				Expect(actualHandle).To(Equal("some-handle"))
			})
		})

		Context("when no worker is found with the container", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns no container, false, and no error", func() {
			})
		})
	})

	Describe("FindOrCreateContainer", func() {
		var (
			ctx                       context.Context
			fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
			metadata                  db.ContainerMetadata
			spec                      ContainerSpec
			workerSpec                WorkerSpec
			resourceTypes             creds.VersionedResourceTypes
			fakeOwner                 *dbfakes.FakeContainerOwner

			fakeContainer *workerfakes.FakeContainer

			createdContainer Container
			createErr        error

			incompatibleWorker *workerfakes.FakeWorker
			compatibleWorker   *workerfakes.FakeWorker
		)

		BeforeEach(func() {
			ctx = context.Background()

			fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

			fakeOwner = new(dbfakes.FakeContainerOwner)

			fakeInput1 := new(workerfakes.FakeInputSource)
			fakeInput1AS := new(workerfakes.FakeArtifactSource)
			fakeInput1AS.VolumeOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
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
			fakeInput2AS.VolumeOnStub = func(logger lager.Logger, worker Worker) (Volume, bool, error) {
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

			variables := template.StaticVariables{
				"secret-source": "super-secret-source",
			}

			resourceTypes = creds.NewVersionedResourceTypes(variables, atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-b",
						Type:   "custom-type-a",
						Source: atc.Source{"some": "((secret-source))"},
					},
					Version: atc.Version{"some": "version"},
				},
			})

			workerSpec = WorkerSpec{
				ResourceType:  "some-type",
				TeamID:        4567,
				Tags:          atc.Tags{"some-tag"},
				ResourceTypes: resourceTypes,
			}

			fakeContainer = new(workerfakes.FakeContainer)

			incompatibleWorker = new(workerfakes.FakeWorker)
			incompatibleWorker.SatisfyingReturns(nil, ErrIncompatiblePlatform)

			compatibleWorker = new(workerfakes.FakeWorker)
			compatibleWorker.SatisfyingReturns(compatibleWorker, nil)
			compatibleWorker.FindOrCreateContainerReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.FindOrCreateContainer(
				ctx,
				logger,
				fakeImageFetchingDelegate,
				fakeOwner,
				metadata,
				spec,
				workerSpec,
				resourceTypes,
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
					workerA.SatisfyingReturns(workerA, nil)
					workerB.SatisfyingReturns(nil, errors.New("nope"))
					workerC.SatisfyingReturns(nil, errors.New("nope"))

					workerA.FindOrCreateContainerReturns(fakeContainer, nil)
				})

				It("checks that the workers satisfy the given worker spec", func() {
					Expect(workerA.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec := workerA.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerB.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec = workerB.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerC.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec = workerC.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))
				})

				It("succeeds and returns the compatible worker with the container", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(0))

					Expect(createErr).NotTo(HaveOccurred())
					Expect(createdContainer).To(Equal(fakeContainer))
				})
			})

			Context("when multiple workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfyingReturns(workerA, nil)
					workerB.SatisfyingReturns(workerB, nil)
					workerC.SatisfyingReturns(nil, errors.New("nope"))

					workerA.FindOrCreateContainerReturns(fakeContainer, nil)
				})

				It("succeeds and returns the first compatible worker with the container", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(0))

					Expect(createErr).NotTo(HaveOccurred())
					Expect(createdContainer).To(Equal(fakeContainer))
				})
			})

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfyingReturns(nil, errors.New("nope"))
					workerB.SatisfyingReturns(nil, errors.New("nope"))
					workerC.SatisfyingReturns(nil, errors.New("nope"))
				})

				It("returns a NoCompatibleWorkersError", func() {
					Expect(createErr).To(Equal(NoCompatibleWorkersError{
						Spec: workerSpec,
					}))
				})
			})

			Context("when the workers that have the containers do not match the workers that satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfyingReturns(workerA, nil)
					workerB.SatisfyingReturns(workerB, nil)
					workerC.SatisfyingReturns(workerC, nil)

					workerD := new(workerfakes.FakeWorker)
					workerD.NameReturns("workerD")

					fakeProvider.FindWorkersForContainerByOwnerReturns([]Worker{workerD}, nil)
				})

				It("creates a new container", func() {
					Expect(fakeStrategy.ChooseCallCount()).To(Equal(1))

					Expect(createErr).NotTo(HaveOccurred())
					Expect(createdContainer).ToNot(Equal(fakeContainer))
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

					workerA.SatisfyingReturns(workerA, nil)
					workerB.SatisfyingReturns(workerB, nil)
					workerC.SatisfyingReturns(nil, errors.New("nope"))

					fakeProvider.RunningWorkersReturns([]Worker{workerA, workerB, workerC}, nil)
					fakeStrategy.ChooseReturns(workerA, nil)
				})

				It("checks that the workers satisfy the given worker spec", func() {
					Expect(workerA.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec := workerA.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerB.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec = workerB.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))

					Expect(workerC.SatisfyingCallCount()).To(Equal(1))
					_, actualSpec = workerC.SatisfyingArgsForCall(0)
					Expect(actualSpec).To(Equal(workerSpec))
				})

				It("returns all workers satisfying the spec", func() {
					_, satisfyingWorkers, _, _ := fakeStrategy.ChooseArgsForCall(0)
					Expect(satisfyingWorkers).To(ConsistOf(workerA, workerB))
				})

				Context("when no workers satisfy the spec", func() {
					BeforeEach(func() {
						workerA.SatisfyingReturns(nil, errors.New("nope"))
						workerB.SatisfyingReturns(nil, errors.New("nope"))
						workerC.SatisfyingReturns(nil, errors.New("nope"))
					})

					It("returns a NoCompatibleWorkersError", func() {
						Expect(createErr).To(Equal(NoCompatibleWorkersError{
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
					teamWorker1.SatisfyingReturns(teamWorker1, nil)
					teamWorker1.IsOwnedByTeamReturns(true)
					teamWorker2 = new(workerfakes.FakeWorker)
					teamWorker2.SatisfyingReturns(teamWorker2, nil)
					teamWorker2.IsOwnedByTeamReturns(true)
					teamWorker3 = new(workerfakes.FakeWorker)
					teamWorker3.SatisfyingReturns(nil, errors.New("nope"))
					generalWorker = new(workerfakes.FakeWorker)
					generalWorker.SatisfyingReturns(generalWorker, nil)
					generalWorker.IsOwnedByTeamReturns(false)
					fakeProvider.RunningWorkersReturns([]Worker{generalWorker, teamWorker1, teamWorker2, teamWorker3}, nil)
					fakeStrategy.ChooseReturns(teamWorker1, nil)
				})

				It("returns only the team workers that satisfy the spec", func() {
					_, satisfyingWorkers, _, _ := fakeStrategy.ChooseArgsForCall(0)
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
					teamWorker.SatisfyingReturns(nil, errors.New("nope"))
					generalWorker1 = new(workerfakes.FakeWorker)
					generalWorker1.SatisfyingReturns(generalWorker1, nil)
					generalWorker1.IsOwnedByTeamReturns(false)
					generalWorker2 = new(workerfakes.FakeWorker)
					generalWorker2.SatisfyingReturns(nil, errors.New("nope"))
					fakeProvider.RunningWorkersReturns([]Worker{generalWorker1, generalWorker2, teamWorker}, nil)
					fakeStrategy.ChooseReturns(generalWorker1, nil)
				})

				It("returns the general workers that satisfy the spec", func() {
					_, satisfyingWorkers, _, _ := fakeStrategy.ChooseArgsForCall(0)
					Expect(satisfyingWorkers).To(ConsistOf(generalWorker1))
				})
			})

			Context("with no workers", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{}, nil)
				})

				It("returns ErrNoWorkers", func() {
					Expect(createErr).To(Equal(ErrNoWorkers))
				})
			})

			Context("when getting the workers fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
				})
			})

			Context("with no workers available", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{}, nil)
				})

				It("returns ErrNoWorkers", func() {
					Expect(createErr).To(Equal(ErrNoWorkers))
				})
			})

			Context("with no compatible workers available", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{incompatibleWorker}, nil)
				})

				It("returns NoCompatibleWorkersError", func() {
					Expect(createErr).To(Equal(NoCompatibleWorkersError{
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
						Expect(createErr).ToNot(HaveOccurred())
						Expect(fakeStrategy.ChooseCallCount()).To(Equal(1))
						Expect(compatibleWorker.FindOrCreateContainerCallCount()).To(Equal(1))
						Expect(createdContainer).To(Equal(fakeContainer))
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
						Expect(createErr).To(Equal(strategyError))
					})
				})
			})
		})
	})
})
