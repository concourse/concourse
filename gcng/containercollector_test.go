package gcng_test

import (
	"errors"

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
		fakeContainerProvider   *gcngfakes.FakeContainerProvider
		fakeGardenClientFactory gcng.GardenClientFactory

		fakeGardenClient *gardenfakes.FakeClient
		logger           *lagertest.TestLogger

		destroyingContainers []*dbng.DestroyingContainer
		fakeWorker1          *dbng.Worker
		fakeWorker2          *dbng.Worker

		gardenAddr string

		gardenClientFactoryCallCount int
		gardenClientFactoryArgs      []*dbng.Worker

		c *gcng.ContainerCollector
	)

	BeforeEach(func() {
		fakeWorkerProvider = new(dbngfakes.FakeWorkerFactory)
		fakeContainerProvider = new(gcngfakes.FakeContainerProvider)

		fakeGardenClient = new(gardenfakes.FakeClient)
		gardenClientFactoryCallCount = 0
		gardenClientFactoryArgs = nil
		fakeGardenClientFactory = func(worker *dbng.Worker) (garden.Client, error) {
			gardenClientFactoryCallCount++
			gardenClientFactoryArgs = append(gardenClientFactoryArgs, worker)

			return fakeGardenClient, nil
		}

		logger = lagertest.NewTestLogger("test")

		gardenAddr = "127.0.0.1"
		fakeWorker1 = &dbng.Worker{
			GardenAddr: &gardenAddr,
			Name:       "foo",
		}
		fakeWorker2 = &dbng.Worker{
			GardenAddr: &gardenAddr,
			Name:       "bar",
		}

		destroyingContainers = []*dbng.DestroyingContainer{
			{ID: 1, Handle: "some-handle", WorkerName: "foo"},
			{ID: 2, Handle: "some-other-handle", WorkerName: "bar"},
		}

		fakeContainerProvider.FindContainersMarkedForDeletionReturns(destroyingContainers, nil)
		fakeWorkerProvider.GetWorkerStub = func(name string) (*dbng.Worker, bool, error) {
			switch name {
			case "foo":
				return fakeWorker1, true, nil
			case "bar":
				return fakeWorker2, true, nil
			default:
				return nil, false, errors.New("no-worker-found")
			}
		}

		fakeContainerProvider.ContainerDestroyReturns(true, nil)

		c = &gcng.ContainerCollector{
			Logger:              logger,
			ContainerProvider:   fakeContainerProvider,
			WorkerProvider:      fakeWorkerProvider,
			GardenClientFactory: fakeGardenClientFactory,
		}
	})

	Describe("Run", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = c.Run()
		})

		It("succeeds", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("marks build containers for deletion", func() {
			Expect(fakeContainerProvider.MarkBuildContainersForDeletionCallCount()).To(Equal(1))
		})

		It("finds all containers in deleting state, tells garden to destroy it, and then removes it from the DB", func() {
			Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))

			Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
			Expect(fakeWorkerProvider.GetWorkerArgsForCall(0)).To(Equal("foo"))
			Expect(fakeWorkerProvider.GetWorkerArgsForCall(1)).To(Equal("bar"))

			Expect(gardenClientFactoryCallCount).To(Equal(2))
			Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker1))
			Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker2))

			Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
			Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle"))
			Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-other-handle"))

			Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(2))
			Expect(fakeContainerProvider.ContainerDestroyArgsForCall(0)).To(Equal(destroyingContainers[0]))
			Expect(fakeContainerProvider.ContainerDestroyArgsForCall(1)).To(Equal(destroyingContainers[1]))
		})

		Context("when marking builds for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerProvider.MarkBuildContainersForDeletionReturns(errors.New("some-error"))
			})

			It("logs the errors and continues", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(2))
			})
		})

		Context("when finding containers marked for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerProvider.FindContainersMarkedForDeletionReturns(nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(0))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(0))
			})
		})

		Context("when getting a worker for a container errors", func() {
			BeforeEach(func() {
				fakeWorkerProvider.GetWorkerStub = func(name string) (*dbng.Worker, bool, error) {
					switch name {
					case "bar":
						return fakeWorker2, true, nil
					default:
						return nil, false, errors.New("no-worker-found")
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(1))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-other-handle"))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(1))
				Expect(fakeContainerProvider.ContainerDestroyArgsForCall(0)).To(Equal(destroyingContainers[1]))

			})

		})

		Context("when a container's worker is not found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.GetWorkerStub = func(name string) (*dbng.Worker, bool, error) {
					switch name {
					case "foo":
						return nil, false, nil
					case "bar":
						return fakeWorker2, true, nil
					default:
						return nil, false, errors.New("no-worker-found")
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(1))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-other-handle"))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(1))
				Expect(fakeContainerProvider.ContainerDestroyArgsForCall(0)).To(Equal(destroyingContainers[1]))
			})
		})

		Context("when getting a garden client for a worker errors", func() {
			BeforeEach(func() {
				fakeGardenClientFactory = func(worker *dbng.Worker) (garden.Client, error) {
					gardenClientFactoryCallCount++
					gardenClientFactoryArgs = append(gardenClientFactoryArgs, worker)

					if gardenClientFactoryCallCount == 1 {
						return nil, errors.New("some-error")
					}

					return fakeGardenClient, nil
				}

				c = &gcng.ContainerCollector{
					Logger:              logger,
					ContainerProvider:   fakeContainerProvider,
					WorkerProvider:      fakeWorkerProvider,
					GardenClientFactory: fakeGardenClientFactory,
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker1))
				Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker2))

				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(1))
				Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-other-handle"))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(1))
				Expect(fakeContainerProvider.ContainerDestroyArgsForCall(0)).To(Equal(destroyingContainers[1]))
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

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))

				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(1))
				Expect(fakeContainerProvider.ContainerDestroyArgsForCall(0)).To(Equal(destroyingContainers[1]))
			})
		})

		Context("when destroying a container in the DB errors", func() {
			BeforeEach(func() {
				fakeContainerProvider.ContainerDestroyStub = func(container *dbng.DestroyingContainer) (bool, error) {
					switch container.Handle {
					case "some-handle":
						return false, errors.New("some-error")
					case "some-other-handle":
						return true, nil
					default:
						return true, nil
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(2))
			})
		})

		Context("when it can't find a container to destroy", func() {
			BeforeEach(func() {
				fakeContainerProvider.ContainerDestroyStub = func(container *dbng.DestroyingContainer) (bool, error) {
					switch container.Handle {
					case "some-handle":
						return false, nil
					case "some-other-handle":
						return true, nil
					default:
						return true, nil
					}
				}
			})

			It("continues destroying the rest of the containers", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeWorkerProvider.GetWorkerCallCount()).To(Equal(2))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(fakeContainerProvider.ContainerDestroyCallCount()).To(Equal(2))
			})
		})
	})
})
