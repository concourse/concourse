package worker_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/db/lock/lockfakes"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/retryhttp/retryhttpfakes"
	"github.com/cppforlife/go-semi-semantic/version"

	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("DBProvider", func() {
	var (
		fakeLockFactory *lockfakes.FakeLockFactory

		logger *lagertest.TestLogger

		fakeGardenBackend                 *gfakes.FakeBackend
		gardenAddr                        string
		baggageclaimURL                   string
		wantWorkerVersion                 version.Version
		baggageclaimServer                *ghttp.Server
		gardenServer                      *server.GardenServer
		provider                          WorkerProvider
		baggageclaimResponseHeaderTimeout time.Duration

		fakeImageFactory                    *workerfakes.FakeImageFactory
		fakeImageFetchingDelegate           *workerfakes.FakeImageFetchingDelegate
		fakeDBVolumeFactory                 *dbfakes.FakeVolumeFactory
		fakeDBWorkerFactory                 *dbfakes.FakeWorkerFactory
		fakeDBTeamFactory                   *dbfakes.FakeTeamFactory
		fakeDBWorkerBaseResourceTypeFactory *dbfakes.FakeWorkerBaseResourceTypeFactory
		fakeDBWorkerTaskCacheFactory        *dbfakes.FakeWorkerTaskCacheFactory
		fakeDBResourceCacheFactory          *dbfakes.FakeResourceCacheFactory
		fakeDBResourceConfigFactory         *dbfakes.FakeResourceConfigFactory
		fakeCreatingContainer               *dbfakes.FakeCreatingContainer
		fakeCreatedContainer                *dbfakes.FakeCreatedContainer

		fakeDBTeam *dbfakes.FakeTeam

		workers    []Worker
		workersErr error

		fakeWorker1 *dbfakes.FakeWorker
		fakeWorker2 *dbfakes.FakeWorker
	)

	BeforeEach(func() {
		var err error
		baggageclaimServer = ghttp.NewServer()

		baggageclaimServer.RouteToHandler("POST", "/volumes", ghttp.RespondWithJSONEncoded(
			http.StatusCreated,
			baggageclaim.VolumeResponse{Handle: "vol-handle"},
		))
		baggageclaimServer.RouteToHandler("PUT", "/volumes/vol-handle/ttl", ghttp.RespondWith(
			http.StatusNoContent,
			nil,
		))
		baggageclaimServer.RouteToHandler("GET", "/volumes/vol-handle", ghttp.RespondWithJSONEncoded(
			http.StatusOK,
			baggageclaim.VolumeResponse{Handle: "vol-handle"},
		))
		baggageclaimServer.RouteToHandler("GET", "/volumes/certificates", ghttp.RespondWithJSONEncoded(
			http.StatusOK,
			baggageclaim.VolumeResponse{Handle: "certificates", Path: "/resource/certs"},
		))
		baggageclaimServer.RouteToHandler("PUT", "/volumes/certificates/stream-out", ghttp.RespondWithJSONEncoded(
			http.StatusOK,
			baggageclaim.VolumeResponse{},
		))

		gardenAddr = fmt.Sprintf("0.0.0.0:%d", 8888+GinkgoParallelNode())
		fakeGardenBackend = new(gfakes.FakeBackend)
		logger = lagertest.NewTestLogger("test")
		gardenServer = server.New("tcp", gardenAddr, 0, fakeGardenBackend, logger)
		baggageclaimResponseHeaderTimeout = 10 * time.Minute

		go func() {
			defer GinkgoRecover()
			err = gardenServer.ListenAndServe()
			Expect(err).NotTo(HaveOccurred())

		}()

		apiClient := client.New(connection.New("tcp", gardenAddr))
		Eventually(apiClient.Ping).Should(Succeed())

		err = gardenServer.SetupBomberman()
		Expect(err).NotTo(HaveOccurred())

		worker1Version := "1.2.3"

		fakeWorker1 = new(dbfakes.FakeWorker)
		fakeWorker1.NameReturns("some-worker")
		fakeWorker1.GardenAddrReturns(&gardenAddr)
		fakeWorker1.BaggageclaimURLReturns(&baggageclaimURL)
		fakeWorker1.StateReturns(db.WorkerStateRunning)
		fakeWorker1.ActiveContainersReturns(2)
		fakeWorker1.ResourceTypesReturns([]atc.WorkerResourceType{
			{Type: "some-resource-a", Image: "some-image-a"}})

		fakeWorker1.VersionReturns(&worker1Version)

		worker2Version := "1.2.4"

		fakeWorker2 = new(dbfakes.FakeWorker)
		fakeWorker2.NameReturns("some-other-worker")
		fakeWorker2.GardenAddrReturns(&gardenAddr)
		fakeWorker2.BaggageclaimURLReturns(&baggageclaimURL)
		fakeWorker2.StateReturns(db.WorkerStateRunning)
		fakeWorker2.ActiveContainersReturns(2)
		fakeWorker2.ResourceTypesReturns([]atc.WorkerResourceType{
			{Type: "some-resource-b", Image: "some-image-b"}})

		fakeWorker2.VersionReturns(&worker2Version)

		fakeImageFactory = new(workerfakes.FakeImageFactory)
		fakeImage := new(workerfakes.FakeImage)
		fakeImage.FetchForContainerReturns(FetchedImage{}, nil)
		fakeImageFactory.GetImageReturns(fakeImage, nil)
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeDBTeamFactory = new(dbfakes.FakeTeamFactory)
		fakeDBTeam = new(dbfakes.FakeTeam)
		fakeDBTeamFactory.GetByIDReturns(fakeDBTeam)
		fakeDBVolumeFactory = new(dbfakes.FakeVolumeFactory)

		fakeBackOffFactory := new(retryhttpfakes.FakeBackOffFactory)
		fakeBackOff := new(retryhttpfakes.FakeBackOff)
		fakeBackOffFactory.NewBackOffReturns(fakeBackOff)
		fakeDBResourceCacheFactory = new(dbfakes.FakeResourceCacheFactory)
		fakeDBResourceConfigFactory = new(dbfakes.FakeResourceConfigFactory)
		fakeDBWorkerBaseResourceTypeFactory = new(dbfakes.FakeWorkerBaseResourceTypeFactory)
		fakeDBWorkerTaskCacheFactory = new(dbfakes.FakeWorkerTaskCacheFactory)
		fakeLock := new(lockfakes.FakeLock)

		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLockFactory.AcquireReturns(fakeLock, true, nil)

		fakeDBWorkerFactory = new(dbfakes.FakeWorkerFactory)

		wantWorkerVersion, err = version.NewVersionFromString("1.1.0")
		Expect(err).ToNot(HaveOccurred())

		provider = NewDBWorkerProvider(
			fakeLockFactory,
			fakeBackOffFactory,
			fakeImageFactory,
			fakeDBResourceCacheFactory,
			fakeDBResourceConfigFactory,
			fakeDBWorkerBaseResourceTypeFactory,
			fakeDBWorkerTaskCacheFactory,
			fakeDBVolumeFactory,
			fakeDBTeamFactory,
			fakeDBWorkerFactory,
			&wantWorkerVersion,
			baggageclaimResponseHeaderTimeout,
		)
		baggageclaimURL = baggageclaimServer.URL()
	})

	AfterEach(func() {
		gardenServer.Stop()

		Eventually(func() error {
			conn, err := net.Dial("tcp", gardenAddr)
			if err == nil {
				conn.Close()
			}

			return err
		}).Should(HaveOccurred())

		baggageclaimServer.Close()
	})

	Describe("RunningWorkers", func() {
		JustBeforeEach(func() {
			workers, workersErr = provider.RunningWorkers(logger)
		})

		Context("when the database yields workers", func() {
			BeforeEach(func() {
				fakeDBWorkerFactory.WorkersReturns([]db.Worker{fakeWorker1, fakeWorker2}, nil)
			})

			It("succeeds", func() {
				Expect(workersErr).NotTo(HaveOccurred())
			})

			It("returns a worker for each one", func() {
				Expect(workers).To(HaveLen(2))
			})

			Context("when some of the workers returned are stalled or landing", func() {
				BeforeEach(func() {
					landingWorker := new(dbfakes.FakeWorker)
					landingWorker.NameReturns("landing-worker")
					landingWorker.GardenAddrReturns(&gardenAddr)
					landingWorker.BaggageclaimURLReturns(&baggageclaimURL)
					landingWorker.StateReturns(db.WorkerStateLanding)
					landingWorker.ActiveContainersReturns(5)
					landingWorker.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					stalledWorker := new(dbfakes.FakeWorker)
					stalledWorker.NameReturns("stalled-worker")
					stalledWorker.GardenAddrReturns(&gardenAddr)
					stalledWorker.BaggageclaimURLReturns(&baggageclaimURL)
					stalledWorker.StateReturns(db.WorkerStateStalled)
					stalledWorker.ActiveContainersReturns(0)
					stalledWorker.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					fakeDBWorkerFactory.WorkersReturns(
						[]db.Worker{
							fakeWorker1,
							stalledWorker,
							landingWorker,
						}, nil)
				})

				It("only returns workers for the running ones", func() {
					Expect(workers).To(HaveLen(1))
					Expect(workersErr).NotTo(HaveOccurred())
				})
			})

			Context("when a worker's major version is higher or lower than the atc worker version", func() {
				BeforeEach(func() {
					worker1 := new(dbfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(db.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1.0"
					worker1.VersionReturns(&version1)

					worker2 := new(dbfakes.FakeWorker)
					worker2.NameReturns("worker-2")
					worker2.GardenAddrReturns(&gardenAddr)
					worker2.BaggageclaimURLReturns(&baggageclaimURL)
					worker2.StateReturns(db.WorkerStateRunning)
					worker2.ActiveContainersReturns(0)
					worker2.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version2 := "2.0.0"
					worker2.VersionReturns(&version2)

					worker3 := new(dbfakes.FakeWorker)
					worker3.NameReturns("worker-2")
					worker3.GardenAddrReturns(&gardenAddr)
					worker3.BaggageclaimURLReturns(&baggageclaimURL)
					worker3.StateReturns(db.WorkerStateRunning)
					worker3.ActiveContainersReturns(0)
					worker3.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version3 := "0.0.0"
					worker3.VersionReturns(&version3)

					fakeDBWorkerFactory.WorkersReturns(
						[]db.Worker{
							worker3,
							worker2,
							worker1,
						}, nil)
				})

				It("only returns workers with same major version", func() {
					Expect(workers).To(HaveLen(1))
					Expect(workers[0].Name()).To(Equal("worker-1"))
					Expect(workersErr).NotTo(HaveOccurred())
				})
			})

			Context("when a worker's minor version is higher or lower than the atc worker version", func() {
				BeforeEach(func() {
					worker1 := new(dbfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(db.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1.0"
					worker1.VersionReturns(&version1)

					worker2 := new(dbfakes.FakeWorker)
					worker2.NameReturns("worker-2")
					worker2.GardenAddrReturns(&gardenAddr)
					worker2.BaggageclaimURLReturns(&baggageclaimURL)
					worker2.StateReturns(db.WorkerStateRunning)
					worker2.ActiveContainersReturns(0)
					worker2.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version2 := "1.2.0"
					worker2.VersionReturns(&version2)

					worker3 := new(dbfakes.FakeWorker)
					worker3.NameReturns("worker-2")
					worker3.GardenAddrReturns(&gardenAddr)
					worker3.BaggageclaimURLReturns(&baggageclaimURL)
					worker3.StateReturns(db.WorkerStateRunning)
					worker3.ActiveContainersReturns(0)
					worker3.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version3 := "1.0.0"
					worker3.VersionReturns(&version3)

					fakeDBWorkerFactory.WorkersReturns(
						[]db.Worker{
							worker3,
							worker2,
							worker1,
						}, nil)
				})

				It("only returns workers with same or higher minor version", func() {
					Expect(workers).To(HaveLen(2))
					Expect(workers[1].Name()).To(Equal("worker-1"))
					Expect(workers[0].Name()).To(Equal("worker-2"))
					Expect(workersErr).NotTo(HaveOccurred())
				})
			})

			Context("when a worker does not have a version (outdated)", func() {
				BeforeEach(func() {
					worker1 := new(dbfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(db.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					fakeDBWorkerFactory.WorkersReturns(
						[]db.Worker{
							worker1,
						}, nil)
				})

				It("does not return the worker", func() {
					Expect(workers).To(BeEmpty())
					Expect(workersErr).NotTo(HaveOccurred())
				})
			})

			Context("when a worker's version is incorretly formatted", func() {
				BeforeEach(func() {
					worker1 := new(dbfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(db.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1..0.2-bogus=version"
					worker1.VersionReturns(&version1)

					fakeDBWorkerFactory.WorkersReturns(
						[]db.Worker{
							worker1,
						}, nil)
				})

				It("does not return the worker", func() {
					Expect(workers).To(BeEmpty())
					Expect(workersErr).NotTo(HaveOccurred())
				})
			})

			Context("creating the connection to garden", func() {
				var spec ContainerSpec

				JustBeforeEach(func() {
					spec = ContainerSpec{
						ImageSpec: ImageSpec{
							ResourceType: "some-resource-a",
						},
					}

					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("created-handle")

					fakeGardenBackend.CreateReturns(fakeContainer, nil)
					fakeGardenBackend.LookupReturns(fakeContainer, nil)

					By("connecting to the worker")
					fakeDBWorkerFactory.GetWorkerReturns(fakeWorker1, true, nil)
					container, err := workers[0].FindOrCreateContainer(logger, nil, fakeImageFetchingDelegate, db.NewBuildStepContainerOwner(42, atc.PlanID("some-plan-id")), db.ContainerMetadata{}, spec, nil)
					Expect(err).NotTo(HaveOccurred())

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					By("restarting the worker with a new address")
					gardenServer.Stop()

					Eventually(func() error {
						conn, err := net.Dial("tcp", gardenAddr)
						if err == nil {
							conn.Close()
						}

						return err
					}).Should(HaveOccurred())

					gardenAddr = fmt.Sprintf("0.0.0.0:%d", 7777+GinkgoParallelNode())

					gardenServer = server.New("tcp", gardenAddr, 0, fakeGardenBackend, logger)
					err = gardenServer.Start()
					Expect(err).NotTo(HaveOccurred())
				})
			})

			Describe("a created container", func() {
				BeforeEach(func() {
					createdVolume := new(dbfakes.FakeCreatedVolume)
					createdVolume.HandleReturns("vol-handle")
					fakeDBWorkerFactory.GetWorkerReturns(fakeWorker1, true, nil)
					fakeDBVolumeFactory.FindContainerVolumeReturns(nil, createdVolume, nil)
					fakeDBVolumeFactory.FindBaseResourceTypeVolumeReturns(nil, createdVolume, nil)

					fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
					fakeCreatingContainer.HandleReturns("some-handle")
					fakeCreatedContainer = new(dbfakes.FakeCreatedContainer)
					fakeCreatingContainer.CreatedReturns(fakeCreatedContainer, nil)
					fakeDBTeam.CreateContainerReturns(fakeCreatingContainer, nil)

					workerBaseResourceType := &db.UsedWorkerBaseResourceType{ID: 42}
					fakeDBWorkerBaseResourceTypeFactory.FindReturns(workerBaseResourceType, true, nil)
				})

				It("calls through to garden", func() {
					spec := ContainerSpec{
						ImageSpec: ImageSpec{
							ResourceType: "some-resource-a",
						},
					}

					fakeContainer := new(gfakes.FakeContainer)
					fakeContainer.HandleReturns("created-handle")

					fakeGardenBackend.CreateReturns(fakeContainer, nil)
					fakeGardenBackend.LookupReturns(fakeContainer, nil)

					container, err := workers[0].FindOrCreateContainer(logger, nil, fakeImageFetchingDelegate, db.NewBuildStepContainerOwner(42, atc.PlanID("some-plan-id")), db.ContainerMetadata{}, spec, nil)
					Expect(err).NotTo(HaveOccurred())

					Expect(container.Handle()).To(Equal("created-handle"))

					Expect(fakeGardenBackend.CreateCallCount()).To(Equal(1))

					err = container.Destroy()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeGardenBackend.DestroyCallCount()).To(Equal(1))
					Expect(fakeGardenBackend.DestroyArgsForCall(0)).To(Equal("created-handle"))
				})
			})
		})

		Context("when the database fails to return workers", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeDBWorkerFactory.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(workersErr).To(Equal(disaster))
			})
		})
	})

	Describe("FindWorkerForContainer", func() {
		var (
			foundWorker Worker
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundWorker, found, findErr = provider.FindWorkerForContainer(
				logger,
				345278,
				"some-handle",
			)
		})

		Context("when the worker is found", func() {
			var fakeExistingWorker *dbfakes.FakeWorker

			BeforeEach(func() {
				addr := "1.2.3.4:7777"

				fakeExistingWorker = new(dbfakes.FakeWorker)
				fakeExistingWorker.NameReturns("some-worker")
				fakeExistingWorker.GardenAddrReturns(&addr)
				workerVersion := "1.1.0"
				fakeExistingWorker.VersionReturns(&workerVersion)

				fakeDBTeam.FindWorkerForContainerReturns(fakeExistingWorker, true, nil)
			})

			It("returns true", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).ToNot(HaveOccurred())
			})

			It("returns the worker", func() {
				Expect(foundWorker).ToNot(BeNil())
				Expect(foundWorker.Name()).To(Equal("some-worker"))
			})

			It("found the worker for the right handle", func() {
				handle := fakeDBTeam.FindWorkerForContainerArgsForCall(0)
				Expect(handle).To(Equal("some-handle"))
			})

			It("found the right team", func() {
				actualTeam := fakeDBTeamFactory.GetByIDArgsForCall(0)
				Expect(actualTeam).To(Equal(345278))
			})

			Context("when the worker version is outdated", func() {
				BeforeEach(func() {
					fakeExistingWorker.VersionReturns(nil)
				})

				It("returns an error", func() {
					Expect(findErr).ToNot(HaveOccurred())
					Expect(foundWorker).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when the worker is not found", func() {
			BeforeEach(func() {
				fakeDBTeam.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns false", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the worker fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeDBTeam.FindWorkerForContainerReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("FindWorkerForContainerByOwner", func() {
		var (
			fakeOwner *dbfakes.FakeContainerOwner

			foundWorker Worker
			found       bool
			findErr     error
		)

		BeforeEach(func() {
			fakeOwner = new(dbfakes.FakeContainerOwner)
		})

		JustBeforeEach(func() {
			foundWorker, found, findErr = provider.FindWorkerForContainerByOwner(
				logger,
				345278,
				fakeOwner,
			)
		})

		Context("when the worker is found", func() {
			var fakeExistingWorker *dbfakes.FakeWorker

			BeforeEach(func() {
				addr := "1.2.3.4:7777"

				fakeExistingWorker = new(dbfakes.FakeWorker)
				fakeExistingWorker.NameReturns("some-worker")
				fakeExistingWorker.GardenAddrReturns(&addr)
				workerVersion := "1.1.0"
				fakeExistingWorker.VersionReturns(&workerVersion)

				fakeDBTeam.FindWorkerForContainerByOwnerReturns(fakeExistingWorker, true, nil)
			})

			It("returns true", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).ToNot(HaveOccurred())
			})

			It("returns the worker", func() {
				Expect(foundWorker).ToNot(BeNil())
				Expect(foundWorker.Name()).To(Equal("some-worker"))
			})

			It("found the worker for the right owner", func() {
				owner := fakeDBTeam.FindWorkerForContainerByOwnerArgsForCall(0)
				Expect(owner).To(Equal(fakeOwner))
			})

			It("found the right team", func() {
				actualTeam := fakeDBTeamFactory.GetByIDArgsForCall(0)
				Expect(actualTeam).To(Equal(345278))
			})

			Context("when the worker version is outdated", func() {
				BeforeEach(func() {
					fakeExistingWorker.VersionReturns(nil)
				})

				It("returns an error", func() {
					Expect(findErr).ToNot(HaveOccurred())
					Expect(foundWorker).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when the worker is not found", func() {
			BeforeEach(func() {
				fakeDBTeam.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns false", func() {
				Expect(findErr).ToNot(HaveOccurred())
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})

		Context("when finding the worker fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeDBTeam.FindWorkerForContainerByOwnerReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})
})
