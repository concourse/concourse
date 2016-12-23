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

		destroyingContainer1 *dbngfakes.FakeDestroyingContainer
		destroyingContainer2 *dbngfakes.FakeDestroyingContainer
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

		destroyingContainer1 = new(dbngfakes.FakeDestroyingContainer)
		destroyingContainer1.HandleReturns("some-handle")
		destroyingContainer1.WorkerNameReturns("foo")
		destroyingContainer2 = new(dbngfakes.FakeDestroyingContainer)
		destroyingContainer2.HandleReturns("some-other-handle")
		destroyingContainer2.WorkerNameReturns("bar")

		fakeContainerProvider.FindContainersMarkedForDeletionReturns([]dbng.DestroyingContainer{
			destroyingContainer1,
			destroyingContainer2,
		}, nil)
		fakeWorkerProvider.WorkersReturns([]*dbng.Worker{fakeWorker1, fakeWorker2}, nil)

		destroyingContainer1.DestroyReturns(true, nil)
		destroyingContainer2.DestroyReturns(true, nil)

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

			Expect(gardenClientFactoryCallCount).To(Equal(2))
			Expect(gardenClientFactoryArgs[0]).To(Equal(fakeWorker1))
			Expect(gardenClientFactoryArgs[1]).To(Equal(fakeWorker2))

			Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
			Expect(fakeGardenClient.DestroyArgsForCall(0)).To(Equal("some-handle"))
			Expect(fakeGardenClient.DestroyArgsForCall(1)).To(Equal("some-other-handle"))

			Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
			Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
		})

		Context("when marking builds for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerProvider.MarkBuildContainersForDeletionReturns(errors.New("some-error"))
			})

			It("logs the errors and continues", func() {
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(gardenClientFactoryCallCount).To(Equal(2))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(2))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(1))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(1))
			})
		})

		Context("when finding containers marked for deletion fails", func() {
			BeforeEach(func() {
				fakeContainerProvider.FindContainersMarkedForDeletionReturns(nil, errors.New("some-error"))
			})

			It("returns and logs the error", func() {
				Expect(err).To(MatchError("some-error"))
				Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
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
				Expect(fakeContainerProvider.FindContainersMarkedForDeletionCallCount()).To(Equal(1))
				Expect(gardenClientFactoryCallCount).To(Equal(0))
				Expect(fakeGardenClient.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer1.DestroyCallCount()).To(Equal(0))
				Expect(destroyingContainer2.DestroyCallCount()).To(Equal(0))
			})
		})

		Context("when a container's worker is not found", func() {
			BeforeEach(func() {
				fakeWorkerProvider.WorkersReturns([]*dbng.Worker{fakeWorker2}, nil)
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
