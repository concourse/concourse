package worker_test

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	gfakes "github.com/cloudfoundry-incubator/garden/fakes"
	"github.com/cloudfoundry-incubator/garden/server"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type immediateRetryPolicy struct{}

func (immediateRetryPolicy) DelayFor(uint) (time.Duration, bool) {
	return 0, true
}

var _ = Describe("DBProvider", func() {
	var (
		fakeDB *fakes.FakeWorkerDB

		logger *lagertest.TestLogger

		worker                *gfakes.FakeBackend
		workerAddr            string
		workerBaggageclaimURL string
		workerServer          *server.GardenServer
		provider              WorkerProvider

		fakeImageFetcher          *fakes.FakeImageFetcher
		fakeImageFetchingDelegate *fakes.FakeImageFetchingDelegate

		workers    []Worker
		workersErr error
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeWorkerDB)
		logger = lagertest.NewTestLogger("test")
		worker = new(gfakes.FakeBackend)

		workerAddr = fmt.Sprintf("0.0.0.0:%d", 8888+GinkgoParallelNode())
		workerBaggageclaimURL = "http://1.2.3.4:7788"

		workerServer = server.New("tcp", workerAddr, 0, worker, logger)
		err := workerServer.Start()
		Expect(err).NotTo(HaveOccurred())

		fakeImageFetcher = new(fakes.FakeImageFetcher)
		fakeImageFetchingDelegate = new(fakes.FakeImageFetchingDelegate)

		provider = NewDBWorkerProvider(logger, fakeDB, nil, immediateRetryPolicy{}, fakeImageFetcher)
	})

	AfterEach(func() {
		workerServer.Stop()

		Eventually(func() error {
			conn, err := net.Dial("tcp", workerAddr)
			if err == nil {
				conn.Close()
			}

			return err
		}).Should(HaveOccurred())
	})

	Context("when we call to get multiple workers", func() {
		JustBeforeEach(func() {
			workers, workersErr = provider.Workers()
		})

		Context("when the database yields workers", func() {
			BeforeEach(func() {
				fakeDB.WorkersReturns([]db.SavedWorker{
					{
						WorkerInfo: db.WorkerInfo{
							Name:             "some-worker",
							GardenAddr:       workerAddr,
							BaggageclaimURL:  workerBaggageclaimURL,
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource-a", Image: "some-image-a"},
							},
						},
					},
					{
						WorkerInfo: db.WorkerInfo{
							Name:             "some-other-worker",
							GardenAddr:       workerAddr,
							ActiveContainers: 2,
							ResourceTypes: []atc.WorkerResourceType{
								{Type: "some-resource-b", Image: "some-image-b"},
							},
						},
					},
				}, nil)
			})

			It("succeeds", func() {
				Expect(workersErr).NotTo(HaveOccurred())
			})

			It("returns a worker for each one", func() {
				Expect(workers).To(HaveLen(2))
			})

			It("constructs workers with baggageclaim clients if they had addresses", func() {
				vm, ok := workers[0].VolumeManager()
				Expect(ok).To(BeTrue())
				Expect(vm).NotTo(BeNil())

				vm, ok = workers[1].VolumeManager()
				Expect(ok).To(BeFalse())
				Expect(vm).To(BeNil())
			})

			Context("creating the connection to garden", func() {
				var id Identifier
				var spec ResourceTypeContainerSpec

				JustBeforeEach(func() {
					id = Identifier{
						ResourceID: 1234,
					}

					spec = ResourceTypeContainerSpec{
						Type: "some-resource-a",
					}

					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("created-handle")

					worker.CreateReturns(fakeContainer, nil)
					worker.LookupReturns(fakeContainer, nil)

					By("connecting to the worker")
					container, err := workers[0].CreateContainer(logger, nil, fakeImageFetchingDelegate, id, Metadata{}, spec, nil)
					Expect(err).NotTo(HaveOccurred())

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("restarting the worker with a new address")
					workerServer.Stop()

					Eventually(func() error {
						conn, err := net.Dial("tcp", workerAddr)
						if err == nil {
							conn.Close()
						}

						return err
					}).Should(HaveOccurred())

					workerAddr = fmt.Sprintf("0.0.0.0:%d", 7777+GinkgoParallelNode())

					workerServer = server.New("tcp", workerAddr, 0, worker, logger)
					err = workerServer.Start()
					Expect(err).NotTo(HaveOccurred())
				})

				It("can continue to connect after the worker address changes", func() {
					fakeDB.GetWorkerReturns(db.SavedWorker{WorkerInfo: db.WorkerInfo{GardenAddr: workerAddr}}, true, nil)

					container, err := workers[0].CreateContainer(logger, nil, fakeImageFetchingDelegate, id, Metadata{}, spec, nil)
					Expect(err).NotTo(HaveOccurred())

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())
				})

				It("throws an error if the worker cannot be found", func() {
					fakeDB.GetWorkerReturns(db.SavedWorker{}, false, nil)

					_, err := workers[0].CreateContainer(logger, nil, fakeImageFetchingDelegate, id, Metadata{}, spec, nil)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(ErrMissingWorker))
				})

				It("throws an error if the lookup of the worker in the db errors", func() {
					expectedErr := errors.New("some-db-error")
					fakeDB.GetWorkerReturns(db.SavedWorker{}, true, expectedErr)

					_, actualErr := workers[0].CreateContainer(logger, nil, fakeImageFetchingDelegate, id, Metadata{}, spec, nil)
					Expect(actualErr).To(HaveOccurred())
					Expect(actualErr).To(Equal(expectedErr))
				})
			})

			Describe("a created container", func() {
				It("calls through to garden", func() {
					id := Identifier{
						ResourceID: 1234,
					}

					spec := ResourceTypeContainerSpec{
						Type: "some-resource-a",
					}

					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("created-handle")

					worker.CreateReturns(fakeContainer, nil)
					worker.LookupReturns(fakeContainer, nil)

					container, err := workers[0].CreateContainer(logger, nil, fakeImageFetchingDelegate, id, Metadata{}, spec, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDB.CreateContainerCallCount()).To(Equal(1))
					createdInfo, _ := fakeDB.CreateContainerArgsForCall(0)
					Expect(createdInfo.WorkerName).To(Equal("some-worker"))

					Expect(container.Handle()).To(Equal("created-handle"))

					Expect(worker.CreateCallCount()).To(Equal(1))

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					Expect(worker.DestroyCallCount()).To(Equal(1))
					Expect(worker.DestroyArgsForCall(0)).To(Equal("created-handle"))
				})
			})

			Describe("a looked-up container", func() {
				It("calls through to garden", func() {
					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("some-handle")

					worker.ContainersReturns([]garden.Container{fakeContainer}, nil)
					worker.LookupReturns(fakeContainer, nil)

					returnContainer := db.Container{
						ContainerMetadata: db.ContainerMetadata{
							Handle: "some-handle",
						},
					}
					fakeDB.FindContainerByIdentifierReturns(returnContainer, true, nil)

					container, found, err := workers[0].FindContainerForIdentifier(logger, Identifier{
						ResourceID: 1234,
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(container.Handle()).To(Equal("some-handle"))

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					Expect(worker.DestroyCallCount()).To(Equal(1))
					Expect(worker.DestroyArgsForCall(0)).To(Equal("some-handle"))
				})
			})
		})

		Context("when the database fails to return workers", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeDB.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(workersErr).To(Equal(disaster))
			})
		})
	})

	Context("when we call to get a single worker", func() {
		var found bool
		var worker Worker

		Context("when looking up workers returns an error", func() {
			It("returns an error", func() {
				fakeDB.GetWorkerReturns(db.SavedWorker{}, true, errors.New("disaster"))

				worker, found, workersErr = provider.GetWorker("a-worker")
				Expect(workersErr).To(HaveOccurred())
				Expect(worker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when we find no workers", func() {
			It("returns found as false", func() {
				fakeDB.GetWorkerReturns(db.SavedWorker{}, false, nil)

				worker, found, workersErr = provider.GetWorker("no-worker")
				Expect(workersErr).NotTo(HaveOccurred())
				Expect(worker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Context("when we call to get a container info by identifier", func() {
		It("calls through to the db object", func() {
			provider.FindContainerForIdentifier(Identifier{
				BuildID: 1234,
				PlanID:  atc.PlanID("planid"),
			})

			Expect(fakeDB.FindContainerByIdentifierCallCount()).To(Equal(1))

			Expect(fakeDB.FindContainerByIdentifierArgsForCall(0)).To(Equal(db.ContainerIdentifier{
				BuildID: 1234,
				PlanID:  atc.PlanID("planid"),
			}))
		})
	})
})
