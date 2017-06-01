package worker_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	gfakes "code.cloudfoundry.org/garden/gardenfakes"
	"code.cloudfoundry.org/garden/server"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/concourse/atc/dbng/dbngfakes"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/workerfakes"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/retryhttp/retryhttpfakes"
	"github.com/cppforlife/go-semi-semantic/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("DBProvider", func() {
	var (
		fakeLockFactory *lockfakes.FakeLockFactory

		logger *lagertest.TestLogger

		fakeGardenBackend  *gfakes.FakeBackend
		gardenAddr         string
		baggageclaimURL    string
		wantWorkerVersion  version.Version
		baggageclaimServer *ghttp.Server
		gardenServer       *server.GardenServer
		provider           WorkerProvider

		fakeImageFactory                    *workerfakes.FakeImageFactory
		fakeImageFetchingDelegate           *workerfakes.FakeImageFetchingDelegate
		fakeDBVolumeFactory                 *dbngfakes.FakeVolumeFactory
		fakeDBWorkerFactory                 *dbngfakes.FakeWorkerFactory
		fakeDBTeamFactory                   *dbngfakes.FakeTeamFactory
		fakeDBWorkerBaseResourceTypeFactory *dbngfakes.FakeWorkerBaseResourceTypeFactory
		fakeDBResourceCacheFactory          *dbngfakes.FakeResourceCacheFactory
		fakeDBResourceConfigFactory         *dbngfakes.FakeResourceConfigFactory
		fakeCreatingContainer               *dbngfakes.FakeCreatingContainer
		fakeCreatedContainer                *dbngfakes.FakeCreatedContainer

		fakeDBTeam *dbngfakes.FakeTeam

		workers    []Worker
		workersErr error

		fakeWorker1 *dbngfakes.FakeWorker
		fakeWorker2 *dbngfakes.FakeWorker
	)

	BeforeEach(func() {
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
		baggageclaimServer.RouteToHandler("GET", "/volumes/vol-handle/stats", ghttp.RespondWithJSONEncoded(
			http.StatusOK,
			baggageclaim.VolumeStatsResponse{SizeInBytes: 1024},
		))

		gardenAddr = fmt.Sprintf("0.0.0.0:%d", 8888+GinkgoParallelNode())
		fakeGardenBackend = new(gfakes.FakeBackend)
		logger = lagertest.NewTestLogger("test")
		gardenServer = server.New("tcp", gardenAddr, 0, fakeGardenBackend, logger)
		err := gardenServer.Start()
		Expect(err).NotTo(HaveOccurred())

		worker1Version := "1.2.3"

		fakeWorker1 = new(dbngfakes.FakeWorker)
		fakeWorker1.NameReturns("some-worker")
		fakeWorker1.GardenAddrReturns(&gardenAddr)
		fakeWorker1.BaggageclaimURLReturns(&baggageclaimURL)
		fakeWorker1.StateReturns(dbng.WorkerStateRunning)
		fakeWorker1.ActiveContainersReturns(2)
		fakeWorker1.ResourceTypesReturns([]atc.WorkerResourceType{
			{Type: "some-resource-a", Image: "some-image-a"}})

		fakeWorker1.VersionReturns(&worker1Version)

		worker2Version := "1.2.4"

		fakeWorker2 = new(dbngfakes.FakeWorker)
		fakeWorker2.NameReturns("some-other-worker")
		fakeWorker2.GardenAddrReturns(&gardenAddr)
		fakeWorker2.BaggageclaimURLReturns(&baggageclaimURL)
		fakeWorker2.StateReturns(dbng.WorkerStateRunning)
		fakeWorker2.ActiveContainersReturns(2)
		fakeWorker2.ResourceTypesReturns([]atc.WorkerResourceType{
			{Type: "some-resource-b", Image: "some-image-b"}})

		fakeWorker2.VersionReturns(&worker2Version)

		fakeImageFactory = new(workerfakes.FakeImageFactory)
		fakeImage := new(workerfakes.FakeImage)
		fakeImage.FetchForContainerReturns(FetchedImage{}, nil)
		fakeImageFactory.GetImageReturns(fakeImage, nil)
		fakeImageFetchingDelegate = new(workerfakes.FakeImageFetchingDelegate)
		fakeDBTeamFactory = new(dbngfakes.FakeTeamFactory)
		fakeDBTeam = new(dbngfakes.FakeTeam)
		fakeDBTeamFactory.GetByIDReturns(fakeDBTeam)
		fakeDBVolumeFactory = new(dbngfakes.FakeVolumeFactory)

		fakeBackOffFactory := new(retryhttpfakes.FakeBackOffFactory)
		fakeBackOff := new(retryhttpfakes.FakeBackOff)
		fakeBackOffFactory.NewBackOffReturns(fakeBackOff)
		fakeDBResourceCacheFactory = new(dbngfakes.FakeResourceCacheFactory)
		fakeDBResourceConfigFactory = new(dbngfakes.FakeResourceConfigFactory)
		fakeDBWorkerBaseResourceTypeFactory = new(dbngfakes.FakeWorkerBaseResourceTypeFactory)
		fakeLock := new(lockfakes.FakeLock)

		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeLockFactory.AcquireReturns(fakeLock, true, nil)

		fakeDBWorkerFactory = new(dbngfakes.FakeWorkerFactory)

		wantWorkerVersion, err = version.NewVersionFromString("1.1.0")
		Expect(err).ToNot(HaveOccurred())

		provider = NewDBWorkerProvider(
			fakeLockFactory,
			fakeBackOffFactory,
			fakeImageFactory,
			fakeDBResourceCacheFactory,
			fakeDBResourceConfigFactory,
			fakeDBWorkerBaseResourceTypeFactory,
			fakeDBVolumeFactory,
			fakeDBTeamFactory,
			fakeDBWorkerFactory,
			&wantWorkerVersion,
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
				fakeDBWorkerFactory.WorkersReturns([]dbng.Worker{fakeWorker1, fakeWorker2}, nil)
			})

			It("succeeds", func() {
				Expect(workersErr).NotTo(HaveOccurred())
			})

			It("returns a worker for each one", func() {
				Expect(workers).To(HaveLen(2))
			})

			Context("when some of the workers returned are stalled or landing", func() {
				BeforeEach(func() {
					landingWorker := new(dbngfakes.FakeWorker)
					landingWorker.NameReturns("landing-worker")
					landingWorker.GardenAddrReturns(&gardenAddr)
					landingWorker.BaggageclaimURLReturns(&baggageclaimURL)
					landingWorker.StateReturns(dbng.WorkerStateLanding)
					landingWorker.ActiveContainersReturns(5)
					landingWorker.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					stalledWorker := new(dbngfakes.FakeWorker)
					stalledWorker.NameReturns("stalled-worker")
					stalledWorker.GardenAddrReturns(&gardenAddr)
					stalledWorker.BaggageclaimURLReturns(&baggageclaimURL)
					stalledWorker.StateReturns(dbng.WorkerStateStalled)
					stalledWorker.ActiveContainersReturns(0)
					stalledWorker.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					fakeDBWorkerFactory.WorkersReturns(
						[]dbng.Worker{
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
					worker1 := new(dbngfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(dbng.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1.0"
					worker1.VersionReturns(&version1)

					worker2 := new(dbngfakes.FakeWorker)
					worker2.NameReturns("worker-2")
					worker2.GardenAddrReturns(&gardenAddr)
					worker2.BaggageclaimURLReturns(&baggageclaimURL)
					worker2.StateReturns(dbng.WorkerStateRunning)
					worker2.ActiveContainersReturns(0)
					worker2.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version2 := "2.0.0"
					worker2.VersionReturns(&version2)

					worker3 := new(dbngfakes.FakeWorker)
					worker3.NameReturns("worker-2")
					worker3.GardenAddrReturns(&gardenAddr)
					worker3.BaggageclaimURLReturns(&baggageclaimURL)
					worker3.StateReturns(dbng.WorkerStateRunning)
					worker3.ActiveContainersReturns(0)
					worker3.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version3 := "0.0.0"
					worker3.VersionReturns(&version3)

					fakeDBWorkerFactory.WorkersReturns(
						[]dbng.Worker{
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
					worker1 := new(dbngfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(dbng.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1.0"
					worker1.VersionReturns(&version1)

					worker2 := new(dbngfakes.FakeWorker)
					worker2.NameReturns("worker-2")
					worker2.GardenAddrReturns(&gardenAddr)
					worker2.BaggageclaimURLReturns(&baggageclaimURL)
					worker2.StateReturns(dbng.WorkerStateRunning)
					worker2.ActiveContainersReturns(0)
					worker2.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version2 := "1.2.0"
					worker2.VersionReturns(&version2)

					worker3 := new(dbngfakes.FakeWorker)
					worker3.NameReturns("worker-2")
					worker3.GardenAddrReturns(&gardenAddr)
					worker3.BaggageclaimURLReturns(&baggageclaimURL)
					worker3.StateReturns(dbng.WorkerStateRunning)
					worker3.ActiveContainersReturns(0)
					worker3.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version3 := "1.0.0"
					worker3.VersionReturns(&version3)

					fakeDBWorkerFactory.WorkersReturns(
						[]dbng.Worker{
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
					worker1 := new(dbngfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(dbng.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})

					fakeDBWorkerFactory.WorkersReturns(
						[]dbng.Worker{
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
					worker1 := new(dbngfakes.FakeWorker)
					worker1.NameReturns("worker-1")
					worker1.GardenAddrReturns(&gardenAddr)
					worker1.BaggageclaimURLReturns(&baggageclaimURL)
					worker1.StateReturns(dbng.WorkerStateRunning)
					worker1.ActiveContainersReturns(5)
					worker1.ResourceTypesReturns([]atc.WorkerResourceType{
						{Type: "some-resource-b", Image: "some-image-b"}})
					version1 := "1.1..0.2-bogus=version"
					worker1.VersionReturns(&version1)

					fakeDBWorkerFactory.WorkersReturns(
						[]dbng.Worker{
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
					container, err := workers[0].FindOrCreateBuildContainer(logger, nil, fakeImageFetchingDelegate, 42, atc.PlanID("some-plan-id"), dbng.ContainerMetadata{}, spec, nil)
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
					createdVolume := new(dbngfakes.FakeCreatedVolume)
					createdVolume.HandleReturns("vol-handle")
					fakeDBWorkerFactory.GetWorkerReturns(fakeWorker1, true, nil)
					fakeDBVolumeFactory.FindContainerVolumeReturns(nil, createdVolume, nil)
					fakeDBVolumeFactory.FindBaseResourceTypeVolumeReturns(nil, createdVolume, nil)

					fakeCreatingContainer = new(dbngfakes.FakeCreatingContainer)
					fakeCreatingContainer.HandleReturns("some-handle")
					fakeCreatedContainer = new(dbngfakes.FakeCreatedContainer)
					fakeCreatingContainer.CreatedReturns(fakeCreatedContainer, nil)
					fakeDBTeam.CreateBuildContainerReturns(fakeCreatingContainer, nil)

					workerBaseResourceType := &dbng.UsedWorkerBaseResourceType{ID: 42}
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

					container, err := workers[0].FindOrCreateBuildContainer(logger, nil, fakeImageFetchingDelegate, 42, atc.PlanID("some-plan-id"), dbng.ContainerMetadata{}, spec, nil)
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
			var fakeExistingWorker *dbngfakes.FakeWorker

			BeforeEach(func() {
				addr := "1.2.3.4:7777"

				fakeExistingWorker = new(dbngfakes.FakeWorker)
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

	Describe("FindWorkerForBuildContainer", func() {
		var (
			foundWorker Worker
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundWorker, found, findErr = provider.FindWorkerForBuildContainer(
				logger,
				345278,
				42,
				atc.PlanID("some-plan-id"),
			)
		})

		Context("when the worker is found", func() {
			var fakeExistingWorker *dbngfakes.FakeWorker

			BeforeEach(func() {
				addr := "1.2.3.4:7777"

				fakeExistingWorker = new(dbngfakes.FakeWorker)
				fakeExistingWorker.NameReturns("some-worker")
				fakeExistingWorker.GardenAddrReturns(&addr)
				workerVersion := "1.1.0"
				fakeExistingWorker.VersionReturns(&workerVersion)

				fakeDBTeam.FindWorkerForBuildContainerReturns(fakeExistingWorker, true, nil)
			})

			It("returns true", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).ToNot(HaveOccurred())
			})

			It("returns the worker", func() {
				Expect(foundWorker).ToNot(BeNil())
				Expect(foundWorker.Name()).To(Equal("some-worker"))
			})

			It("found the worker for the right resource config", func() {
				actualBuildID, actualPlanID := fakeDBTeam.FindWorkerForBuildContainerArgsForCall(0)
				Expect(actualBuildID).To(Equal(42))
				Expect(actualPlanID).To(Equal(atc.PlanID("some-plan-id")))
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
				fakeDBTeam.FindWorkerForBuildContainerReturns(nil, false, nil)
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
				fakeDBTeam.FindWorkerForBuildContainerReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})

	Describe("FindWorkerForResourceCheckContainer", func() {
		var (
			foundWorker Worker
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundWorker, found, findErr = provider.FindWorkerForResourceCheckContainer(
				logger,
				345278,
				dbng.ForResource(1235),
				"some-resource-type",
				atc.Source{"some": "source"},
				atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Type:   "some-custom-type",
							Source: atc.Source{"some": "custom-source"},
						},
						Version: atc.Version{"some": "custom-version"},
					},
				},
			)
		})

		Context("when creating the resource config succeeds", func() {
			var usedResourceConfig *dbng.UsedResourceConfig

			BeforeEach(func() {
				usedResourceConfig = &dbng.UsedResourceConfig{ID: 1}

				fakeDBResourceConfigFactory.FindOrCreateResourceConfigReturns(usedResourceConfig, nil)
			})

			Context("when the worker is found", func() {
				var fakeExistingWorker *dbngfakes.FakeWorker

				BeforeEach(func() {
					addr := "1.2.3.4:7777"

					fakeExistingWorker = new(dbngfakes.FakeWorker)
					fakeExistingWorker.NameReturns("some-worker")
					fakeExistingWorker.GardenAddrReturns(&addr)
					workerVersion := "1.1.0"
					fakeExistingWorker.VersionReturns(&workerVersion)

					fakeDBTeam.FindWorkerForResourceCheckContainerReturns(fakeExistingWorker, true, nil)
				})

				It("returns true", func() {
					Expect(found).To(BeTrue())
					Expect(findErr).ToNot(HaveOccurred())
				})

				It("returns the worker", func() {
					Expect(foundWorker).ToNot(BeNil())
					Expect(foundWorker.Name()).To(Equal("some-worker"))
				})

				It("found the worker for the right resource config", func() {
					actualConfig := fakeDBTeam.FindWorkerForResourceCheckContainerArgsForCall(0)
					Expect(actualConfig).To(Equal(usedResourceConfig))
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
					fakeDBTeam.FindWorkerForResourceCheckContainerReturns(nil, false, nil)
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
					fakeDBTeam.FindWorkerForResourceCheckContainerReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Expect(findErr).To(Equal(disaster))
					Expect(foundWorker).To(BeNil())
					Expect(found).To(BeFalse())
				})
			})
		})

		Context("when creating the resource config fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeDBResourceConfigFactory.FindOrCreateResourceConfigReturns(nil, disaster)
			})

			It("returns the error", func() {
				Expect(findErr).To(Equal(disaster))
				Expect(foundWorker).To(BeNil())
				Expect(found).To(BeFalse())
			})
		})
	})
})
