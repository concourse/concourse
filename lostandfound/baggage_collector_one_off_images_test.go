package lostandfound_test

import (
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/lostandfoundfakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
)

var _ = Describe("Baggage-collecting image resource volumes created by one-off builds", func() {

	var (
		fakeWorkerClient *wfakes.FakeClient
		worker1          *wfakes.FakeWorker

		worker2 *wfakes.FakeWorker
		volume2 *wfakes.FakeVolume

		fakeBaggageCollectorDB *lostandfoundfakes.FakeBaggageCollectorDB
		fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory
		fakeBuild1             *dbfakes.FakeBuild
		fakeBuild2             *dbfakes.FakeBuild
		fakeBuild4             *dbfakes.FakeBuild
		fakeBuild5             *dbfakes.FakeBuild

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
		volume2 = new(wfakes.FakeVolume)
		worker2.LookupVolumeReturns(volume2, true, nil)

		workerMap := map[string]*wfakes.FakeWorker{
			"worker1": worker1,
			"worker2": worker2,
		}

		fakeWorkerClient.GetWorkerStub = func(name string) (worker.Worker, error) {
			return workerMap[name], nil
		}
		baggageCollectorLogger := lagertest.NewTestLogger("test")

		fakeBaggageCollectorDB = new(lostandfoundfakes.FakeBaggageCollectorDB)
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

		fakeBaggageCollectorDB.GetAllPipelinesReturns([]db.SavedPipeline{savedPipeline}, nil)

		savedVolumes := []db.SavedVolume{
			{
				Volume: db.Volume{
					WorkerName: "worker1",
					TTL:        expectedOneOffTTL,
					Handle:     "volume1",
					Identifier: db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceVersion: atc.Version{"digest": "digest1"},
							ResourceHash:    `docker:{"repository":"repository1"}`,
						},
					},
				},
			},
			{
				Volume: db.Volume{
					WorkerName: "worker2",
					TTL:        expectedOneOffTTL,
					Handle:     "volume2",
					Identifier: db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceVersion: atc.Version{"digest": "digest2"},
							ResourceHash:    `docker:{"repository":"repository2"}`,
						},
					},
				},
			},
		}
		fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)
		fakeBaggageCollectorDB.GetVolumesForOneOffBuildImageResourcesReturns(savedVolumes, nil)

		identifier1 := db.ResourceCacheIdentifier{
			ResourceVersion: atc.Version{"digest": "digest1"},
			ResourceHash:    `docker:{"repository":"repository1"}`,
		}
		identifier2 := db.ResourceCacheIdentifier{
			ResourceVersion: atc.Version{"digest": "digest2"},
			ResourceHash:    `docker:{"repository":"repository2"}`,
		}

		fakeBuild1 = new(dbfakes.FakeBuild)
		fakeBuild1.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{identifier1}, nil)
		fakeBuild2 = new(dbfakes.FakeBuild)
		fakeBuild2.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{identifier2}, nil)
		fakeBuild3 := new(dbfakes.FakeBuild)
		fakeBuild3.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{}, nil)
		fakeBuild4 = new(dbfakes.FakeBuild)
		fakeBuild4.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{identifier1}, nil)
		fakeBuild5 = new(dbfakes.FakeBuild)
		fakeBuild5.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{identifier2}, nil)

		fakePipelineDB = new(dbfakes.FakePipelineDB)
		fakePipelineDB.GetJobFinishedAndNextBuildReturns(fakeBuild2, fakeBuild3, nil)

		fakePipelineDBFactory.BuildReturns(fakePipelineDB)
	})

	It("sets the ttl of each volume used in a one-off build to at least 24 hours", func() {
		err := baggageCollector.Run()
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeBaggageCollectorDB.GetAllPipelinesCallCount()).To(Equal(1))
		Expect(fakePipelineDBFactory.BuildCallCount()).To(Equal(1))
		Expect(fakePipelineDBFactory.BuildArgsForCall(0)).To(Equal(savedPipeline))
		Expect(fakePipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))
		Expect(fakePipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)).To(Equal("my-precious-job"))
		Expect(fakeBuild2.GetImageResourceCacheIdentifiersCallCount()).To(Equal(1))
		Expect(fakeBaggageCollectorDB.GetVolumesForOneOffBuildImageResourcesCallCount()).To(Equal(1))
		Expect(fakeBaggageCollectorDB.GetVolumesCallCount()).To(Equal(1))
		Expect(fakeWorkerClient.GetWorkerCallCount()).To(Equal(2))

		Expect(worker2.LookupVolumeCallCount()).To(Equal(1))
		_, handle := worker2.LookupVolumeArgsForCall(0)
		Expect(handle).To(Equal("volume2"))
		Expect(volume2.ReleaseCallCount()).To(Equal(1))
		Expect(volume2.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedLatestVersionTTL)))
	})
})
