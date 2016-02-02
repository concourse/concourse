package lostandfound_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/fakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/fakes"
	bcfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Baggage-collecting image resource volumes created by one-off builds", func() {

	var (
		fakeWorkerClient *wfakes.FakeClient
		worker1          *wfakes.FakeWorker

		worker2             *wfakes.FakeWorker
		baggageClaimClient2 *bcfakes.FakeClient
		volume2             *bcfakes.FakeVolume

		fakeBaggageCollectorDB *fakes.FakeBaggageCollectorDB
		fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory

		expectedOldVersionTTL    = 4 * time.Minute
		expectedLatestVersionTTL = time.Duration(0)
		expectedOneOffTTL        = 5 * time.Hour

		baggageCollector lostandfound.BaggageCollector

		savedPipeline  db.SavedPipeline
		fakePipelineDB *dbfakes.FakePipelineDB
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)

		worker1 = new(wfakes.FakeWorker)

		worker2 = new(wfakes.FakeWorker)
		baggageClaimClient2 = new(bcfakes.FakeClient)
		worker2.VolumeManagerReturns(baggageClaimClient2, true)
		volume2 = new(bcfakes.FakeVolume)
		baggageClaimClient2.LookupVolumeReturns(volume2, true, nil)

		workerMap := map[string]*wfakes.FakeWorker{
			"worker1": worker1,
			"worker2": worker2,
		}

		fakeWorkerClient.GetWorkerStub = func(name string) (worker.Worker, error) {
			return workerMap[name], nil
		}
		baggageCollectorLogger := lagertest.NewTestLogger("test")

		fakeBaggageCollectorDB = new(fakes.FakeBaggageCollectorDB)
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

		baggageCollector = lostandfound.NewBaggageCollector(
			baggageCollectorLogger,
			fakeWorkerClient,
			fakeBaggageCollectorDB,
			fakePipelineDBFactory,
			expectedOldVersionTTL,
			expectedOneOffTTL,
		)

		savedPipeline = db.SavedPipeline{
			Pipeline: db.Pipeline{
				Name: "my-special-pipeline",
				Config: atc.Config{
					Groups:    atc.GroupConfigs{},
					Resources: atc.ResourceConfigs{},
					Jobs: atc.JobConfigs{
						{
							Name: "my-precious-job",
						},
					},
				},
			},
		}

		fakeBaggageCollectorDB.GetAllActivePipelinesReturns([]db.SavedPipeline{savedPipeline}, nil)

		savedVolumes := []db.SavedVolume{
			{
				Volume: db.Volume{
					WorkerName: "worker1",
					TTL:        expectedOneOffTTL,
					Handle:     "volume1",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"digest": "digest1"},
						ResourceHash:    `docker:{"repository":"repository1"}`,
					},
				},
			},
			{
				Volume: db.Volume{
					WorkerName: "worker2",
					TTL:        expectedOneOffTTL,
					Handle:     "volume2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"digest": "digest2"},
						ResourceHash:    `docker:{"repository":"repository2"}`,
					},
				},
			},
		}
		fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)
		fakeBaggageCollectorDB.GetVolumesForOneOffBuildImageResourcesReturns(savedVolumes, nil)

		identifier1 := db.VolumeIdentifier{
			ResourceVersion: atc.Version{"digest": "digest1"},
			ResourceHash:    `docker:{"repository":"repository1"}`,
		}
		identifier2 := db.VolumeIdentifier{
			ResourceVersion: atc.Version{"digest": "digest2"},
			ResourceHash:    `docker:{"repository":"repository2"}`,
		}
		imageVersionMap := map[int][]db.VolumeIdentifier{
			1: {identifier1},
			2: {identifier2},
			4: {identifier1},
			5: {identifier2},
		}

		fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDStub = func(buildID int) ([]db.VolumeIdentifier, error) {
			return imageVersionMap[buildID], nil
		}

		fakePipelineDB = new(dbfakes.FakePipelineDB)
		fakePipelineDB.GetJobFinishedAndNextBuildReturns(&db.Build{ID: 2}, &db.Build{ID: 3}, nil)

		fakePipelineDBFactory.BuildReturns(fakePipelineDB)
	})

	It("sets the ttl of each volume used in a one-off build to at least 24 hours", func() {
		err := baggageCollector.Collect()
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeBaggageCollectorDB.GetAllActivePipelinesCallCount()).To(Equal(1))
		Expect(fakePipelineDBFactory.BuildCallCount()).To(Equal(1))
		Expect(fakePipelineDBFactory.BuildArgsForCall(0)).To(Equal(savedPipeline))
		Expect(fakePipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))
		Expect(fakePipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)).To(Equal("my-precious-job"))
		Expect(fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDCallCount()).To(Equal(1))
		Expect(fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDArgsForCall(0)).To(Equal(2))
		Expect(fakeBaggageCollectorDB.GetVolumesForOneOffBuildImageResourcesCallCount()).To(Equal(1))
		Expect(fakeBaggageCollectorDB.GetVolumesCallCount()).To(Equal(1))
		Expect(fakeWorkerClient.GetWorkerCallCount()).To(Equal(2))

		Expect(worker1.VolumeManagerCallCount()).To(Equal(0))
		Expect(worker2.VolumeManagerCallCount()).To(Equal(1))

		Expect(baggageClaimClient2.LookupVolumeCallCount()).To(Equal(1))
		_, handle := baggageClaimClient2.LookupVolumeArgsForCall(0)
		Expect(handle).To(Equal("volume2"))
		Expect(volume2.ReleaseCallCount()).To(Equal(1))
		Expect(volume2.ReleaseArgsForCall(0)).To(Equal(expectedLatestVersionTTL))

		Expect(fakeBaggageCollectorDB.SetVolumeTTLCallCount()).To(Equal(1))
		handle, ttl := fakeBaggageCollectorDB.SetVolumeTTLArgsForCall(0)
		Expect(handle).To(Equal("volume2"))
		Expect(ttl).To(Equal(expectedLatestVersionTTL))
	})
})
