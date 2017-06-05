package worker_test

import (
	"errors"
	"os"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		logger       *lagertest.TestLogger
		fakeProvider *workerfakes.FakeWorkerProvider

		pool Client
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakeProvider = new(workerfakes.FakeWorkerProvider)

		pool = NewPool(fakeProvider)
	})

	Describe("Satisfying", func() {
		var (
			spec WorkerSpec

			satisfyingErr    error
			satisfyingWorker Worker
			resourceTypes    atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{
				Platform: "some-platform",
				Tags:     []string{"step", "tags"},
			}
			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-resource-type",
						Type:   "some-underlying-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}
		})

		JustBeforeEach(func() {
			satisfyingWorker, satisfyingErr = pool.Satisfying(logger, spec, resourceTypes)
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
				_, actualSpec, actualResourceTypes := workerA.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerB.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerB.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerC.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerC.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
			})

			It("returns a random worker satisfying the spec", func() {
				chosenCount := map[Worker]int{workerA: 0, workerB: 0, workerC: 0}
				for i := 0; i < 100; i++ {
					satisfyingWorker, satisfyingErr = pool.Satisfying(logger, spec, resourceTypes)
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
						Spec:    spec,
						Workers: []Worker{workerA, workerB, workerC},
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

	Describe("AllSatisfying", func() {
		var (
			spec WorkerSpec

			satisfyingErr     error
			satisfyingWorkers []Worker
			resourceTypes     atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			spec = WorkerSpec{
				Platform: "some-platform",
				Tags:     []string{"step", "tags"},
			}
			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "some-resource-type",
						Type:   "some-underlying-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}
		})

		JustBeforeEach(func() {
			satisfyingWorkers, satisfyingErr = pool.AllSatisfying(logger, spec, resourceTypes)
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
				_, actualSpec, actualResourceTypes := workerA.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerB.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerB.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerC.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerC.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
			})

			It("returns all workers satisfying the spec", func() {
				satisfyingWorkers, satisfyingErr = pool.AllSatisfying(logger, spec, resourceTypes)
				Expect(satisfyingErr).NotTo(HaveOccurred())
				Expect(satisfyingWorkers).To(ConsistOf(workerA, workerB))
			})

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfyingReturns(nil, errors.New("nope"))
					workerB.SatisfyingReturns(nil, errors.New("nope"))
					workerC.SatisfyingReturns(nil, errors.New("nope"))
				})

				It("returns a NoCompatibleWorkersError", func() {
					Expect(satisfyingErr).To(Equal(NoCompatibleWorkersError{
						Spec:    spec,
						Workers: []Worker{workerA, workerB, workerC},
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
			})

			It("returns only the team workers that satisfy the spec", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
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
			})

			It("returns the general workers that satisfy the spec", func() {
				Expect(satisfyingErr).NotTo(HaveOccurred())
				Expect(satisfyingWorkers).To(ConsistOf(generalWorker1))
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

	Describe("FindOrCreateBuildContainer", func() {
		var (
			signals                   <-chan os.Signal
			fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate
			metadata                  db.ContainerMetadata
			spec                      ContainerSpec
			resourceTypes             atc.VersionedResourceTypes

			fakeContainer *workerfakes.FakeContainer

			createdContainer Container
			createErr        error

			incompatibleWorker        *workerfakes.FakeWorker
			compatibleWorkerOneCache1 *workerfakes.FakeWorker
			compatibleWorkerOneCache2 *workerfakes.FakeWorker
			compatibleWorkerTwoCaches *workerfakes.FakeWorker
			compatibleWorkerNoCaches1 *workerfakes.FakeWorker
			compatibleWorkerNoCaches2 *workerfakes.FakeWorker
		)

		BeforeEach(func() {
			fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

			fakeInput1 := new(workerfakes.FakeInputSource)
			fakeInput1AS := new(workerfakes.FakeArtifactSource)
			fakeInput1AS.VolumeOnStub = func(worker Worker) (Volume, bool, error) {
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
			fakeInput2AS.VolumeOnStub = func(worker Worker) (Volume, bool, error) {
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
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}

			fakeContainer = new(workerfakes.FakeContainer)

			incompatibleWorker = new(workerfakes.FakeWorker)
			incompatibleWorker.SatisfyingReturns(nil, ErrIncompatiblePlatform)

			compatibleWorkerOneCache1 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache1.SatisfyingReturns(compatibleWorkerOneCache1, nil)
			compatibleWorkerOneCache1.FindOrCreateBuildContainerReturns(fakeContainer, nil)

			compatibleWorkerOneCache2 = new(workerfakes.FakeWorker)
			compatibleWorkerOneCache2.SatisfyingReturns(compatibleWorkerOneCache2, nil)
			compatibleWorkerOneCache2.FindOrCreateBuildContainerReturns(fakeContainer, nil)

			compatibleWorkerTwoCaches = new(workerfakes.FakeWorker)
			compatibleWorkerTwoCaches.SatisfyingReturns(compatibleWorkerTwoCaches, nil)
			compatibleWorkerTwoCaches.FindOrCreateBuildContainerReturns(fakeContainer, nil)

			compatibleWorkerNoCaches1 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches1.SatisfyingReturns(compatibleWorkerNoCaches1, nil)
			compatibleWorkerNoCaches1.FindOrCreateBuildContainerReturns(fakeContainer, nil)

			compatibleWorkerNoCaches2 = new(workerfakes.FakeWorker)
			compatibleWorkerNoCaches2.SatisfyingReturns(compatibleWorkerNoCaches2, nil)
			compatibleWorkerNoCaches2.FindOrCreateBuildContainerReturns(fakeContainer, nil)
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.FindOrCreateBuildContainer(
				logger,
				signals,
				fakeImageFetchingDelegate,
				42,
				atc.PlanID("some-plan-id"),
				metadata,
				spec,
				resourceTypes,
			)
		})

		Context("when a worker is found with the container", func() {
			var fakeWorker *workerfakes.FakeWorker

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForBuildContainerReturns(fakeWorker, true, nil)
				fakeWorker.FindOrCreateBuildContainerReturns(fakeContainer, nil)
			})

			It("succeeds", func() {
				Expect(createErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(createdContainer).To(Equal(fakeContainer))
			})

			It("'find-or-create's on the particular worker", func() {
				Expect(fakeWorker.FindOrCreateBuildContainerCallCount()).To(Equal(1))

				_, actualTeamID, actualBuildID, actualPlanID := fakeProvider.FindWorkerForBuildContainerArgsForCall(0)
				Expect(actualTeamID).To(Equal(4567))
				Expect(actualBuildID).To(Equal(42))
				Expect(actualPlanID).To(Equal(atc.PlanID("some-plan-id")))
			})
		})

		Context("when no worker is found with the container", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForBuildContainerReturns(nil, false, nil)
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
						Spec:    spec.WorkerSpec(),
						Workers: []Worker{incompatibleWorker},
					}))
				})
			})

			Context("with compatible workers available, with one having the most local caches", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{
						incompatibleWorker,
						compatibleWorkerOneCache1,
						compatibleWorkerTwoCaches,
						compatibleWorkerNoCaches1,
						compatibleWorkerNoCaches2,
					}, nil)
				})

				It("creates it on the worker with the most caches", func() {
					Expect(createErr).ToNot(HaveOccurred())
					Expect(compatibleWorkerTwoCaches.FindOrCreateBuildContainerCallCount()).To(Equal(1))
					Expect(createdContainer).To(Equal(fakeContainer))
				})
			})

			Context("with compatible workers available, with multiple with the same amount of local caches", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{
						incompatibleWorker,
						compatibleWorkerOneCache1,
						compatibleWorkerOneCache2,
						compatibleWorkerNoCaches1,
						compatibleWorkerNoCaches2,
					}, nil)
				})

				It("creates it on a random one of the two", func() {
					Expect(createErr).ToNot(HaveOccurred())
					Expect(createdContainer).To(Equal(fakeContainer))

					for i := 0; i < 100; i++ {
						container, err := pool.FindOrCreateBuildContainer(
							logger,
							signals,
							fakeImageFetchingDelegate,
							42,
							atc.PlanID("some-plan-id"),
							metadata,
							spec,
							resourceTypes,
						)
						Expect(err).ToNot(HaveOccurred())
						Expect(container).To(Equal(fakeContainer))
					}

					Expect(compatibleWorkerOneCache1.FindOrCreateBuildContainerCallCount()).ToNot(BeZero())
					Expect(compatibleWorkerOneCache2.FindOrCreateBuildContainerCallCount()).ToNot(BeZero())
					Expect(compatibleWorkerNoCaches1.FindOrCreateBuildContainerCallCount()).To(BeZero())
					Expect(compatibleWorkerNoCaches2.FindOrCreateBuildContainerCallCount()).To(BeZero())
				})
			})

			Context("with compatible workers available, with none having any local caches", func() {
				BeforeEach(func() {
					fakeProvider.RunningWorkersReturns([]Worker{
						incompatibleWorker,
						compatibleWorkerNoCaches1,
						compatibleWorkerNoCaches2,
					}, nil)
				})

				It("creates it on a random one of them", func() {
					Expect(createErr).ToNot(HaveOccurred())
					Expect(createdContainer).To(Equal(fakeContainer))

					for i := 0; i < 100; i++ {
						container, err := pool.FindOrCreateBuildContainer(
							logger,
							signals,
							fakeImageFetchingDelegate,
							42,
							atc.PlanID("some-plan-id"),
							metadata,
							spec,
							resourceTypes,
						)
						Expect(err).ToNot(HaveOccurred())
						Expect(container).To(Equal(fakeContainer))
					}

					Expect(incompatibleWorker.FindOrCreateBuildContainerCallCount()).To(BeZero())
					Expect(compatibleWorkerNoCaches1.FindOrCreateBuildContainerCallCount()).ToNot(BeZero())
					Expect(compatibleWorkerNoCaches2.FindOrCreateBuildContainerCallCount()).ToNot(BeZero())
				})
			})
		})
	})

	Describe("FindOrCreateResourceCheckContainer", func() {
		var (
			fakeImageFetchingDelegate *workerfakes.FakeImageFetchingDelegate

			spec ContainerSpec

			fakeContainer *workerfakes.FakeContainer

			createdContainer Container
			createErr        error
			resourceTypes    atc.VersionedResourceTypes
		)

		BeforeEach(func() {
			fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)

			spec = ContainerSpec{
				ImageSpec: ImageSpec{ResourceType: "some-type"},
				TeamID:    4567,
			}

			resourceTypes = atc.VersionedResourceTypes{
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-b",
						Type:   "custom-type-a",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-a",
						Type:   "some-resource",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-c",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "custom-type-d",
						Type:   "custom-type-b",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
				{
					ResourceType: atc.ResourceType{
						Name:   "unknown-custom-type",
						Type:   "unknown-base-type",
						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
				},
			}
		})

		JustBeforeEach(func() {
			createdContainer, createErr = pool.FindOrCreateResourceCheckContainer(
				logger,
				db.ForBuild(42),
				make(chan os.Signal),
				fakeImageFetchingDelegate,
				db.ContainerMetadata{},
				spec,
				resourceTypes,
				"some-type",
				atc.Source{"some": "source"},
			)
		})

		Context("when a worker is found with the container", func() {
			var fakeWorker *workerfakes.FakeWorker

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForResourceCheckContainerReturns(fakeWorker, true, nil)
				fakeWorker.FindOrCreateResourceCheckContainerReturns(fakeContainer, nil)
			})

			It("succeeds", func() {
				Expect(createErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(createdContainer).To(Equal(fakeContainer))
			})

			It("'find-or-create's on the particular worker", func() {
				Expect(fakeWorker.FindOrCreateResourceCheckContainerCallCount()).To(Equal(1))

				_, actualTeamID, actualResourceUser, actualResourceType, actualResourceSource, actualResourceTypes := fakeProvider.FindWorkerForResourceCheckContainerArgsForCall(0)
				Expect(actualTeamID).To(Equal(4567))
				Expect(actualResourceUser).To(Equal(db.ForBuild(42)))
				Expect(actualResourceType).To(Equal("some-type"))
				Expect(actualResourceSource).To(Equal(atc.Source{"some": "source"}))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
			})
		})

		Context("when a worker is not found, and multiple are present", func() {
			var (
				workerA *workerfakes.FakeWorker
				workerB *workerfakes.FakeWorker
				workerC *workerfakes.FakeWorker
			)

			BeforeEach(func() {
				fakeProvider.FindWorkerForResourceCheckContainerReturns(nil, false, nil)

				workerA = new(workerfakes.FakeWorker)
				workerB = new(workerfakes.FakeWorker)
				workerC = new(workerfakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				workerA.SatisfyingReturns(workerA, nil)
				workerB.SatisfyingReturns(workerB, nil)
				workerC.SatisfyingReturns(nil, errors.New("nope"))

				workerA.FindOrCreateResourceCheckContainerReturns(fakeContainer, nil)
				workerB.FindOrCreateResourceCheckContainerReturns(fakeContainer, nil)
				workerC.FindOrCreateResourceCheckContainerReturns(fakeContainer, nil)

				fakeProvider.RunningWorkersReturns([]Worker{workerA, workerB, workerC}, nil)
			})

			It("succeeds", func() {
				Expect(createErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(createdContainer).To(Equal(fakeContainer))
			})

			It("checks that the workers satisfy the given spec", func() {
				Expect(workerA.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes := workerA.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec.WorkerSpec()))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerB.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerB.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec.WorkerSpec()))
				Expect(actualResourceTypes).To(Equal(resourceTypes))

				Expect(workerC.SatisfyingCallCount()).To(Equal(1))
				_, actualSpec, actualResourceTypes = workerC.SatisfyingArgsForCall(0)
				Expect(actualSpec).To(Equal(spec.WorkerSpec()))
				Expect(actualResourceTypes).To(Equal(resourceTypes))
			})

			It("creates using a random worker", func() {
				for i := 1; i < 100; i++ { // account for initial create in JustBefore
					createdContainer, createErr := pool.FindOrCreateResourceCheckContainer(
						logger,
						db.ForBuild(42),
						make(chan os.Signal),
						fakeImageFetchingDelegate,
						db.ContainerMetadata{},
						spec,
						resourceTypes,
						"some-type",
						atc.Source{"some": "source"},
					)
					Expect(createErr).NotTo(HaveOccurred())
					Expect(createdContainer).To(Equal(fakeContainer))
				}

				Expect(workerA.FindOrCreateResourceCheckContainerCallCount()).To(BeNumerically("~", workerB.FindOrCreateResourceCheckContainerCallCount(), 50))
				Expect(workerC.FindOrCreateResourceCheckContainerCallCount()).To(BeZero())
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerA.FindOrCreateResourceCheckContainerReturns(nil, disaster)
					workerB.FindOrCreateResourceCheckContainerReturns(nil, disaster)
				})

				It("returns the error", func() {
					Expect(createErr).To(Equal(disaster))
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
						Spec:    spec.WorkerSpec(),
						Workers: []Worker{workerA, workerB, workerC},
					}))

				})
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
	})
})
