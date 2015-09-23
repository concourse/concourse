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
		Ω(err).ShouldNot(HaveOccurred())

		err = workerBServer.Start()
		Ω(err).ShouldNot(HaveOccurred())

		provider = NewDBWorkerProvider(logger, fakeDB, nil)
	})

	JustBeforeEach(func() {
		workers, workersErr = provider.Workers()
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

	Context("when the database yields workers", func() {
		BeforeEach(func() {
			fakeDB.WorkersReturns([]db.WorkerInfo{
				{
					GardenAddr:       workerAAddr,
					BaggageclaimURL:  workerABaggageclaimURL,
					ActiveContainers: 2,
					ResourceTypes: []atc.WorkerResourceType{
						{Type: "some-resource-a", Image: "some-image-a"},
					},
				},
				{
					GardenAddr:       workerBAddr,
					BaggageclaimURL:  workerBBaggageclaimURL,
					ActiveContainers: 2,
					ResourceTypes: []atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"},
					},
				},
			}, nil)
		})

		It("succeeds", func() {
			Ω(workersErr).ShouldNot(HaveOccurred())
		})

		It("returns a worker for each one", func() {
			Ω(workers).Should(HaveLen(2))
		})

		It("constructs workers with baggageclaim clients if they had addresses", func() {
			vm, ok := workers[0].VolumeManager()
			Ω(ok).Should(BeTrue())
			Ω(vm).ShouldNot(BeNil())

			vm, ok = workers[1].VolumeManager()
			Ω(ok).Should(BeFalse())
			Ω(vm).Should(BeNil())
		})

		Describe("a created container", func() {
			It("calls through to garden", func() {
				id := Identifier{Name: "some-name"}

				spec := ResourceTypeContainerSpec{
					Type: "some-resource-a",
				}

				fakeContainer := new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("created-handle")

				workerA.CreateReturns(fakeContainer, nil)
				workerA.LookupReturns(fakeContainer, nil)

				container, err := workers[0].CreateContainer(id, spec)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(container.Handle()).Should(Equal("created-handle"))

				Ω(workerA.CreateCallCount()).Should(Equal(1))
				Ω(workerA.CreateArgsForCall(0).Properties).Should(Equal(garden.Properties{
					"concourse:name": "some-name",
				}))

				err = container.Destroy()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(workerA.DestroyCallCount()).Should(Equal(1))
				Ω(workerA.DestroyArgsForCall(0)).Should(Equal("created-handle"))
			})
		})

		Describe("a looked-up container", func() {
			It("calls through to garden", func() {
				fakeContainer := new(gfakes.FakeContainer)
				fakeContainer.HandleReturns("some-handle")

				workerA.ContainersReturns([]garden.Container{fakeContainer}, nil)
				workerA.LookupReturns(fakeContainer, nil)

				container, err := workers[0].FindContainerForIdentifier(Identifier{Name: "some-name"})
				Ω(err).ShouldNot(HaveOccurred())

				Ω(container.Handle()).Should(Equal("some-handle"))

				err = container.Destroy()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(workerA.DestroyCallCount()).Should(Equal(1))
				Ω(workerA.DestroyArgsForCall(0)).Should(Equal("some-handle"))
			})
		})
	})

	Context("when the database fails to return workers", func() {
		disaster := errors.New("nope")

		BeforeEach(func() {
			fakeDB.WorkersReturns(nil, disaster)
		})

		It("returns the error", func() {
			Ω(workersErr).Should(Equal(disaster))
		})
	})
})
