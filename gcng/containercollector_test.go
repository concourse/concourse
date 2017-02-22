package gcng_test

import (
	"errors"
	"time"

	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/gcng"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc/dbng/dbngfakes"
	"github.com/concourse/atc/gcng/gcngfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerCollector", func() {
	var (
		fakeWorkerProvider      *dbngfakes.FakeWorkerFactory
		fakeContainerFactory    *gcngfakes.FakeContainerFactory
		fakeGardenClientFactory gcng.GardenClientFactory

		fakeGardenClient *gardenfakes.FakeClient
		logger           *lagertest.TestLogger

		destroyingContainer1 *dbngfakes.FakeDestroyingContainer
		destroyingContainer2 *dbngfakes.FakeDestroyingContainer
		fakeWorker1          *dbngfakes.FakeWorker
		fakeWorker2          *dbngfakes.FakeWorker

		gardenAddr string

		gardenClientFactoryCallCount int
		gardenClientFactoryArgs      []dbng.Worker

		collector gcng.Collector
	)

	BeforeEach(func() {
		fakeWorkerProvider = new(dbngfakes.FakeWorkerFactory)
		fakeContainerFactory = new(gcngfakes.FakeContainerFactory)

		fakeGardenClient = new(gardenfakes.FakeClient)
		gardenClientFactoryCallCount = 0
		gardenClientFactoryArgs = nil
		fakeGardenClientFactory = func(worker dbng.Worker) (garden.Client, error) {
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

		destroyingContainer1 = new(dbngfakes.FakeDestroyingContainer)
		destroyingContainer1.HandleReturns("some-handle")
		destroyingContainer1.WorkerNameReturns("foo")
		destroyingContainer2 = new(dbngfakes.FakeDestroyingContainer)
		destroyingContainer2.HandleReturns("some-other-handle")
		destroyingContainer2.WorkerNameReturns("bar")

		fakeContainerFactory.FindContainersMarkedForDeletionReturns([]dbng.DestroyingContainer{
			destroyingContainer1,
			destroyingContainer2,
		}, nil)
		fakeWorkerProvider.WorkersReturns([]dbng.Worker{fakeWorker1, fakeWorker2}, nil)

		destroyingContainer1.DestroyReturns(true, nil)
		destroyingContainer2.DestroyReturns(true, nil)

		collector = gcng.NewContainerCollector(
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

		It("marks build containers for deletion", func() {
			Expect(fakeContainerFactory.MarkContainersForDeletionCallCount()).To(Equal(1))
		})

		Context("when there are created containers in hijacked state", func() {
			var (
				fakeGardenContainer *gardenfakes.FakeContainer
				fakeContainer       *dbngfakes.FakeCreatedContainer
			)

			BeforeEach(func() {
				fakeContainer = new(dbngfakes.FakeCreatedContainer)
				fakeContainer.WorkerNameReturns("foo")
				fakeGardenContainer = new(gardenfakes.FakeContainer)
				fakeContainerFactory.FindHijackedContainersForDeletionReturns([]dbng.CreatedContainer{fakeContainer}, nil)
			})

			Context("when container still exists in garden", func() {
				BeforeEach(func() {
					fakeContainer.HandleReturns("im-fake-and-still-hijacked")
					fakeGardenClient.LookupReturns(fakeGardenContainer, nil)
				})

				It("tells garden to set the TTL to 5 Min", func() {
					Expect(fakeGardenClient.LookupCallCount()).To(Equal(1))
					lookupHandle := fakeGardenClient.LookupArgsForCall(0)
					Expect(lookupHandle).To(Equal("im-fake-and-still-hijacked"))

					Expect(fakeGardenContainer.SetGraceTimeCallCount()).To(Equal(1))
					graceTime := fakeGardenContainer.SetGraceTimeArgsForCall(0)
					Expect(graceTime).To(Equal(5 * time.Minute))
				})

				It("marks container as discontinued in database", func() {
					Expect(fakeContainer.DiscontinueCallCount()).To(Equal(1))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{Handle: "im-fake-and-still-hijacked"})
				})

				It("marks container as destroying", func() {
					Expect(fakeContainer.DestroyingCallCount()).To(Equal(1))
				})
			})
		})

		It("finds all containers in destroying state, tells garden to destroy it, and then removes it from the DB", func() {
			Expect(fakeContainerFactory.FindContainersMarkedForDeletionCallCount()).To(Equal(1))

			Expect(gardenClientFactoryCallCount).To(Equal(2))
			Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker1))
			Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker2))

			Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
			Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle"))
			Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-other-handle"))

			Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
			Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
		})

		Context("when there are destroying containers that are discontinued", func() {
			BeforeEach(func() {
				destroyingContainer1.IsDiscontinuedReturns(true)
				destroyingContainer2.IsDiscontinuedReturns(true)
			})

			Context("when container exists in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(new(gardenfakes.FakeContainer), nil)
				})

				It("does not delete container and lets it expire in garden first", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
					Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
					Expect(destroyingContainer2.DestroyCallCount()).To(Equal(0))
				})
			})

			Context("when container does not exist in garden", func() {
				BeforeEach(func() {
					fakeGardenClient.LookupReturns(nil, garden.ContainerNotFoundError{})
				})

				It("deletes container in database", func() {
					Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
					Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
					Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
				})
			})
		})

		Context("when marking builds for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerFactory.MarkContainersForDeletionReturns(errors.New("some-error"))
			})

			It("logs the errors and continues", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeContainerFactory.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when finding containers marked for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerFactory.FindContainersMarkedForDeletionReturns(nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerFactory.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when getting workers errors", func() {
			BeforeEach(func() {
				fakeWorkerProvider.WorkersReturns(nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerFactory.FindHijackedContainersForDeletionCallCount()).To(Equal(0))
				Expect(fakeContainerFactory.FindContainersMarkedForDeletionCallCount()).To(Equal(0))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(0))
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
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-other-handle"))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when getting a garden client for a worker errors", func() {
			BeforeEach(func() {
				fakeGardenClientFactory = func(worker dbng.Worker) (garden.Client, error) {
					gardenClientFactoryCallCount++
					gardenClientFactoryArgs = append(gardenClientFactoryArgs, worker)

					if gardenClientFactoryCallCount == 1 {
						return nil, errors.New("some-error")
					}

					return fakeGardenClient, nil
				}

				collector = gcng.NewContainerCollector(
					logger,
					fakeContainerFactory,
					fakeWorkerProvider,
					fakeGardenClientFactory,
				)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker1))
				Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker2))

				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-other-handle"))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a garden container errors", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle":
						return errors.New("some-error")
					case "some-other-handle":
						return nil
					default:
						return nil
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))

				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a garden container errors because container is not found", func() {
			BeforeEach(func() {
				fakeGardenClient.DestroyStub = func(handle string) error {
					switch handle {
					case "some-handle":
						return garden.ContainerNotFoundError{Handle: "some-handle"}
					case "some-other-handle":
						return nil
					default:
						return nil
					}
				}
			})

			It("deletes container from database", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))

				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when destroying a container in the DB errors", func() {
			BeforeEach(func() {
				destroyingContainer1.DestroyReturns(false, errors.New("some-error"))
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when it can't find a container to destroy", func() {
			BeforeEach(func() {
				destroyingContainer1.DestroyReturns(false, nil)
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})
	})
})
