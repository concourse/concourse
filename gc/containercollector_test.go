package gc_test

import (
	"errors"
	"time"

	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gc"
	"github.com/concourse/atc/gc/gcfakes"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerCollector", func() {
	var (
		fakeWorkerProvider      *dbngfakes.FakeWorkerFactory
		fakeContainerFactory    *gcfakes.FakeContainerFactory
		fakeGardenClientFactory gc.GardenClientFactory

		fakeGardenClient *gardenfakes.FakeClient
		logger           *lagertest.TestLogger

		creatingContainer               *dbngfakes.FakeCreatingContainer
		createdContainerFromCreating    *dbngfakes.FakeCreatedContainer
		destroyingContainerFromCreating *dbngfakes.FakeDestroyingContainer

		createdContainer               *dbngfakes.FakeCreatedContainer
		destroyingContainerFromCreated *dbngfakes.FakeDestroyingContainer

		destroyingContainer *dbngfakes.FakeDestroyingContainer

		fakeWorker1 *dbngfakes.FakeWorker
		fakeWorker2 *dbngfakes.FakeWorker

		gardenAddr string

		gardenClientFactoryCallCount int
		gardenClientFactoryArgs      []dbng.Worker

		collector gc.Collector
	)

	BeforeEach(func() {
		fakeWorkerProvider = new(dbngfakes.FakeWorkerFactory)
		fakeContainerFactory = new(gcfakes.FakeContainerFactory)

		fakeGardenClient = new(gardenfakes.FakeClient)
		gardenClientFactoryCallCount = 0
		gardenClientFactoryArgs = nil
		fakeGardenClientFactory = func(worker dbng.Worker, logger lager.Logger) (garden.Client, error) {
			gardenClientFactoryCallCount++
			gardenClientFactoryArgs = append(gardenClientFactoryArgs, worker)

			return fakeGardenClient, nil
		}

		logger = lagertest.NewTestLogger("test")

		gardenAddr = "127.0.0.1"

		fakeWorker1 = new(dbngfakes.FakeWorker)
		fakeWorker1.NameReturns("foo")
		fakeWorker1.GardenAddrReturns(&gardenAddr)

		fakeWorker2 = new(dbngfakes.FakeWorker)
		fakeWorker2.NameReturns("bar")
		fakeWorker2.GardenAddrReturns(&gardenAddr)

		creatingContainer = new(dbngfakes.FakeCreatingContainer)
		creatingContainer.HandleReturns("some-handle-1")

		createdContainerFromCreating = new(dbngfakes.FakeCreatedContainer)
		creatingContainer.CreatedReturns(createdContainerFromCreating, nil)
		createdContainerFromCreating.HandleReturns("some-handle-1")

		destroyingContainerFromCreating = new(dbngfakes.FakeDestroyingContainer)
		createdContainerFromCreating.DestroyingReturns(destroyingContainerFromCreating, nil)
		destroyingContainerFromCreating.HandleReturns("some-handle-1")
		destroyingContainerFromCreating.WorkerNameReturns("foo")

		createdContainer = new(dbngfakes.FakeCreatedContainer)
		createdContainer.HandleReturns("some-handle-2")
		createdContainer.WorkerNameReturns("foo")

		destroyingContainerFromCreated = new(dbngfakes.FakeDestroyingContainer)
		createdContainer.DestroyingReturns(destroyingContainerFromCreated, nil)
		destroyingContainerFromCreated.HandleReturns("some-handle-2")
		destroyingContainerFromCreated.WorkerNameReturns("foo")

		destroyingContainer = new(dbngfakes.FakeDestroyingContainer)
		destroyingContainer.HandleReturns("some-handle-3")
		destroyingContainer.WorkerNameReturns("bar")

		fakeContainerFactory.FindContainersForDeletionReturns(
			[]dbng.CreatingContainer{
				creatingContainer,
			},
			[]dbng.CreatedContainer{
				createdContainer,
			},
			[]dbng.DestroyingContainer{
				destroyingContainer,
			},
			nil,
		)
		fakeWorkerProvider.WorkersReturns([]dbng.Worker{fakeWorker1, fakeWorker2}, nil)

		destroyingContainerFromCreating.DestroyReturns(true, nil)
		destroyingContainerFromCreated.DestroyReturns(true, nil)
		destroyingContainer.DestroyReturns(true, nil)

		collector = gc.NewContainerCollector(
			logger,
			fakeContainerFactory,
			fakeWorkerProvider,
			fakeGardenClientFactory,
		)
	})

	Describe("Run", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = collector.Run()
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are created containers in hijacked state", func() {
			var (
				fakeGardenContainer *gardenfakes.FakeContainer
			)

			BeforeEach(func() {
				createdContainer.IsHijackedReturns(true)
				fakeGardenContainer = new(gardenfakes.FakeContainer)
			})

			Context("when container still exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
				})

				It("tells garden to set the TTL to 5 Min", func() {
					Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
					lookupHandle := fakeGardenClient.LookupArgsForCall(0)
					Expect(lookupHandle).To(Equal("some-handle-2"))

					Expect(fakeGardenContainer.SetGraceTimeCallCount()).To(Equal(1))
					graceTime := fakeGardenContainer.SetGraceTimeArgsForCall(0)
					Expect(graceTime).To(Equal(5 * time.Minute))
				})

				It("marks container as discontinued in database", func() {
					Expect(createdContainer.DiscontinueCallCount()).To(Equal(1))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "im-fake-and-still-hijacked"})
				})

				It("marks container as destroying", func() {
					Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
				})
			})
		})

		It("marks all found containers as destroying, tells garden to destroy it, and then removes it from the DB", func() {
			Expect(fakeContainerFactory.FindContainersForDeletionCallCount()).To(Equal(1))

			Expect(creatingContainer.CreatedCallCount()).To(Equal(1))
			Expect(createdContainerFromCreating.DestroyingCallCount()).To(Equal(1))
			Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(1))

			Expect(createdContainer.DestroyingCallCount()).To(Equal(1))
			Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))

			Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))

			Expect(gardenClientFactoryCallCount).To(Equal(3))
			Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
			Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker1))
			Expect(gardenClientFactoryArgs[2]).To(Equal(fakeWorker1))

			Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
			Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-3"))
			Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-2"))
			Expect(fakeGardenClient.DestroyArgsForCall(2)).To(Equal("some-handle-1"))
		})

		Context("when there are destroying containers that are discontinued", func() {
			BeforeEach(func() {
				destroyingContainer.IsDiscontinuedReturns(true)
			})

			Context("when container exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(new(gardenfakes.FakeContainer), nil)
				})

				It("does not delete container and lets it expire in garden first", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
					Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))

					Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{})
				})

				It("deletes container in database", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
					Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
					Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))

					Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
				})
			})
		})

		Context("when finding containers for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerFactory.FindContainersForDeletionReturns(nil, nil, nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerFactory.FindContainersForDeletionCallCount()).To(Equal(1))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when getting workers errors", func() {
			BeforeEach(func() {
				fakeWorkerProvider.WorkersReturns(nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerFactory.FindContainersForDeletionCallCount()).To(Equal(0))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when a container's worker is not found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.WorkersReturns([]dbng.Worker{fakeWorker2}, nil)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(1))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-3"))
				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when a container's worker is stalled", func() {
			BeforeEach(func() {
				fakeWorker1.StateReturns(dbng.WorkerStateStalled)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(1))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-3"))
				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when a container's worker is landed", func() {
			BeforeEach(func() {
				fakeWorker1.StateReturns(dbng.WorkerStateLanded)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(1))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-3"))
				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when getting a garden client for a worker errors", func() {
			BeforeEach(func() {
				fakeGardenClientFactory = func(worker dbng.Worker, logger lager.Logger) (garden.Client, error) {
					gardenClientFactoryCallCount++
					gardenClientFactoryArgs = append(gardenClientFactoryArgs, worker)

					if gardenClientFactoryCallCount == 1 {
						return nil, errors.New("some-error")
					}

					return fakeGardenClient, nil
				}

				collector = gc.NewContainerCollector(
					logger,
					fakeContainerFactory,
					fakeWorkerProvider,
					fakeGardenClientFactory,
				)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(3))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker1))
				Expect(gardenClientFactoryArgs[2]).To(Equal(fakeWorker1))

				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle-2"))
				Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-handle-1"))
				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when destroying a garden container errors", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle-1":
						return errors.New("some-error")
					case "some-handle-2":
						return nil
					case "some-handle-3":
						return nil
					default:
						return nil
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))

				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a garden container errors because container is not found", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle-1":
						return garden.ContainerNotFoundError{Handle: "some-handle"}
					case "some-handle-2":
						return nil
					case "some-handle-3":
						return nil
					default:
						return nil
					}
				}
			})

			It("deletes container from database", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))

				Expect(destroyingContainerFromCreating.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a container in the DB errors", func() {
			BeforeEach(func() {
				destroyingContainerFromCreating.DestroyReturns(false, errors.New("some-error"))
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when it can't find a container to destroy", func() {
			BeforeEach(func() {
				destroyingContainerFromCreating.DestroyReturns(false, nil)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(3))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(3))
				Expect(destroyingContainerFromCreated.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer.DestroyCallCount()).To(Equal(1))
			})
		})
	})
})
