package worker_test

import (
	"errors"
	"fmt"
	"net"

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

var _ = Describe("DBProvider", func() {
	var (
		fakeDB *fakes.FakeWorkerDB

		logger *lagertest.TestLogger

		workerA *gfakes.FakeBackend
		workerB *gfakes.FakeBackend

		workerAAddr string
		workerBAddr string

		workerABaggageclaimURL string
		workerBBaggageclaimURL string

		workerAServer *server.GardenServer
		workerBServer *server.GardenServer

		provider WorkerProvider

		workers    []Worker
		workersErr error
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeWorkerDB)

		logger = lagertest.NewTestLogger("test")

		workerA = new(gfakes.FakeBackend)
		workerB = new(gfakes.FakeBackend)

		workerAAddr = fmt.Sprintf("0.0.0.0:%d", 8888+GinkgoParallelNode())
		workerBAddr = fmt.Sprintf("0.0.0.0:%d", 9999+GinkgoParallelNode())

		workerABaggageclaimURL = "http://1.2.3.4:7788"
		workerBBaggageclaimURL = ""

		workerAServer = server.New("tcp", workerAAddr, 0, workerA, logger)
		workerBServer = server.New("tcp", workerBAddr, 0, workerB, logger)

		err := workerAServer.Start()
		Expect(err).NotTo(HaveOccurred())

		err = workerBServer.Start()
		Expect(err).NotTo(HaveOccurred())

		provider = NewDBWorkerProvider(logger, fakeDB, nil)
	})

	AfterEach(func() {
		workerAServer.Stop()
		workerBServer.Stop()

		Eventually(func() error {
			conn, err := net.Dial("tcp", workerAAddr)
			if err == nil {
				conn.Close()
			}

			return err
		}).Should(HaveOccurred())

		Eventually(func() error {
			conn, err := net.Dial("tcp", workerBAddr)
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
				fakeDB.WorkersReturns([]db.WorkerInfo{
					{
						Name:             "some-worker-name",
						GardenAddr:       workerAAddr,
						BaggageclaimURL:  workerABaggageclaimURL,
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource-a", Image: "some-image-a"},
						},
					},
					{
						GardenAddr:       workerAAddr,
						ActiveContainers: 2,
						ResourceTypes: []atc.WorkerResourceType{
							{Type: "some-resource-b", Image: "some-image-b"},
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

			Describe("a created container", func() {
				It("calls through to garden", func() {
					id := Identifier{
						Name: "some-name",
					}

					spec := ResourceTypeContainerSpec{
						Type: "some-resource-a",
					}

					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("created-handle")

					workerA.CreateReturns(fakeContainer, nil)
					workerA.LookupReturns(fakeContainer, nil)

					container, err := workers[0].CreateContainer(logger, id, spec)
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeDB.CreateContainerInfoCallCount()).To(Equal(1))
					createdInfo, _ := fakeDB.CreateContainerInfoArgsForCall(0)
					Expect(createdInfo.WorkerName).To(Equal("some-worker-name"))

					Expect(container.Handle()).To(Equal("created-handle"))

					Expect(workerA.CreateCallCount()).To(Equal(1))

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					Expect(workerA.DestroyCallCount()).To(Equal(1))
					Expect(workerA.DestroyArgsForCall(0)).To(Equal("created-handle"))
				})
			})

			Describe("a looked-up container", func() {
				It("calls through to garden", func() {
					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("some-handle")

					workerA.ContainersReturns([]garden.Container{fakeContainer}, nil)
					workerA.LookupReturns(fakeContainer, nil)

					fakeDB.FindContainerInfoByIdentifierReturns(db.ContainerInfo{Handle: "some-handle"}, true, nil)

					container, found, err := workers[0].FindContainerForIdentifier(logger, Identifier{
						Name: "some-name",
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(container.Handle()).To(Equal("some-handle"))

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					Expect(workerA.DestroyCallCount()).To(Equal(1))
					Expect(workerA.DestroyArgsForCall(0)).To(Equal("some-handle"))
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
				fakeDB.GetWorkerReturns(db.WorkerInfo{}, true, errors.New("disaster"))

				worker, found, workersErr = provider.GetWorker("some-name")
				Expect(workersErr).To(HaveOccurred())
				Expect(worker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when we find no workers", func() {
			It("returns found as false", func() {
				fakeDB.GetWorkerReturns(db.WorkerInfo{}, false, nil)

				worker, found, workersErr = provider.GetWorker("some-name")
				Expect(workersErr).NotTo(HaveOccurred())
				Expect(worker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Context("when we call to get a container info by identifier", func() {
		It("calls through to the db object", func() {
			provider.FindContainerInfoForIdentifier(Identifier{
				Name:         "some-name",
				PipelineName: "some-pipeline",
				BuildID:      1234,
				Type:         db.ContainerTypePut,
				CheckType:    "some-check-type",
				CheckSource:  atc.Source{"some": "source"},
				WorkerName:   "some-worker-name",
				StepLocation: 1,
			})

			Ω(fakeDB.FindContainerInfoByIdentifierCallCount()).Should(Equal(1))

			Ω(fakeDB.FindContainerInfoByIdentifierArgsForCall(0)).Should(Equal(db.ContainerIdentifier{
				Name:         "some-name",
				PipelineName: "some-pipeline",
				BuildID:      1234,
				Type:         db.ContainerTypePut,
				CheckType:    "some-check-type",
				CheckSource:  atc.Source{"some": "source"},
				WorkerName:   "some-worker-name",
				StepLocation: 1,
			}))
		})
	})
})
