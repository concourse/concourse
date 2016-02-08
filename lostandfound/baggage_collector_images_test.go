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

var _ = Describe("Baggage-collecting image resource volumes", func() {
	Context("when there is a single job", func() {
		var (
			fakeWorkerClient *wfakes.FakeClient
			workerA          *wfakes.FakeWorker

			workerB                   *wfakes.FakeWorker
			workerBBaggageClaimClient *bcfakes.FakeClient
			dockerVolume              *bcfakes.FakeVolume

			workerC                   *wfakes.FakeWorker
			workerCBaggageClaimClient *bcfakes.FakeClient
			crossedWiresVolume        *bcfakes.FakeVolume

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

			workerA = new(wfakes.FakeWorker)

			workerB = new(wfakes.FakeWorker)
			workerBBaggageClaimClient = new(bcfakes.FakeClient)
			workerB.VolumeManagerReturns(workerBBaggageClaimClient, true)
			dockerVolume = new(bcfakes.FakeVolume)
			workerBBaggageClaimClient.LookupVolumeReturns(dockerVolume, true, nil)

			workerC = new(wfakes.FakeWorker)
			workerCBaggageClaimClient = new(bcfakes.FakeClient)
			workerC.VolumeManagerReturns(workerCBaggageClaimClient, true)
			crossedWiresVolume = new(bcfakes.FakeVolume)
			workerCBaggageClaimClient.LookupVolumeReturns(crossedWiresVolume, true, nil)

			workerMap := map[string]*wfakes.FakeWorker{
				"workerA": workerA,
				"workerB": workerB,
				"workerC": workerC,
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

			imageVersionMap := map[int][]db.VolumeIdentifier{
				2: {
					{
						ResourceVersion: atc.Version{"ref": "rence"},
						ResourceHash:    "git:zxcvbnm",
					},
					{
						ResourceVersion: atc.Version{"digest": "readers"},
						ResourceHash:    "docker:qwertyuiop",
					},
				},
				3: {
					{
						ResourceVersion: atc.Version{"ref": "rence"},
						ResourceHash:    "docker:qwertyuiop",
					},
				},
			}

			fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDStub = func(buildID int) ([]db.VolumeIdentifier, error) {
				return imageVersionMap[buildID], nil
			}

			fakePipelineDB = new(dbfakes.FakePipelineDB)
			fakePipelineDB.GetJobFinishedAndNextBuildReturns(&db.Build{ID: 2}, &db.Build{ID: 3}, nil)

			fakePipelineDBFactory.BuildReturns(fakePipelineDB)

			savedVolumes := []db.SavedVolume{
				{
					Volume: db.Volume{
						WorkerName: "workerA",
						TTL:        expectedLatestVersionTTL,
						Handle:     "git-volume-handle",
						VolumeIdentifier: db.VolumeIdentifier{
							ResourceVersion: atc.Version{"ref": "rence"},
							ResourceHash:    "git:zxcvbnm",
						},
					},
				},
				{
					Volume: db.Volume{
						WorkerName: "workerB",
						TTL:        expectedOldVersionTTL,
						Handle:     "docker-volume-handle",
						VolumeIdentifier: db.VolumeIdentifier{
							ResourceVersion: atc.Version{"digest": "readers"},
							ResourceHash:    "docker:qwertyuiop",
						},
					},
				},
				{
					Volume: db.Volume{
						WorkerName: "workerC",
						TTL:        92 * time.Minute,
						Handle:     "crossed-wires-volume-handle",
						VolumeIdentifier: db.VolumeIdentifier{
							ResourceVersion: atc.Version{"ref": "rence"},
							ResourceHash:    "docker:qwertyuiop",
						},
					},
				},
			}
			fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)
		})

		It("preserves only the image versions used by the latest finished build of each job", func() {
			err := baggageCollector.Collect()
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBaggageCollectorDB.GetAllActivePipelinesCallCount()).To(Equal(1))
			Expect(fakePipelineDBFactory.BuildCallCount()).To(Equal(1))
			Expect(fakePipelineDBFactory.BuildArgsForCall(0)).To(Equal(savedPipeline))
			Expect(fakePipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))
			Expect(fakePipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)).To(Equal("my-precious-job"))
			Expect(fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDCallCount()).To(Equal(1))
			Expect(fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDArgsForCall(0)).To(Equal(2))
			Expect(fakeBaggageCollectorDB.GetVolumesCallCount()).To(Equal(1))
			Expect(fakeWorkerClient.GetWorkerCallCount()).To(Equal(3))

			Expect(workerA.VolumeManagerCallCount()).To(Equal(0))
			Expect(workerB.VolumeManagerCallCount()).To(Equal(1))
			Expect(workerC.VolumeManagerCallCount()).To(Equal(1))

			var handle string
			Expect(workerBBaggageClaimClient.LookupVolumeCallCount()).To(Equal(1))
			_, handle = workerBBaggageClaimClient.LookupVolumeArgsForCall(0)
			Expect(handle).To(Equal("docker-volume-handle"))
			Expect(dockerVolume.ReleaseCallCount()).To(Equal(1))
			Expect(dockerVolume.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedLatestVersionTTL)))

			Expect(workerCBaggageClaimClient.LookupVolumeCallCount()).To(Equal(1))
			_, handle = workerCBaggageClaimClient.LookupVolumeArgsForCall(0)
			Expect(handle).To(Equal("crossed-wires-volume-handle"))
			Expect(crossedWiresVolume.ReleaseCallCount()).To(Equal(1))
			Expect(crossedWiresVolume.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedOldVersionTTL)))

			Expect(fakeBaggageCollectorDB.SetVolumeTTLCallCount()).To(Equal(2))

			type setVolumeTTLArgs struct {
				Handle string
				TTL    time.Duration
			}

			expectedSetVolumeTTLArgs := []setVolumeTTLArgs{
				{
					Handle: "docker-volume-handle",
					TTL:    expectedLatestVersionTTL,
				},
				{
					Handle: "crossed-wires-volume-handle",
					TTL:    expectedOldVersionTTL,
				},
			}

			var actualSetVolumeTTLArgs []setVolumeTTLArgs
			for i := range expectedSetVolumeTTLArgs {
				handle, ttl := fakeBaggageCollectorDB.SetVolumeTTLArgsForCall(i)
				actualSetVolumeTTLArgs = append(actualSetVolumeTTLArgs, setVolumeTTLArgs{
					Handle: handle,
					TTL:    ttl,
				})
			}

			Expect(actualSetVolumeTTLArgs).To(ConsistOf(expectedSetVolumeTTLArgs))
		})

		Context("When the job has no previously finished builds", func() {
			var (
				workerABaggageClaimClient *bcfakes.FakeClient
				gitVolume                 *bcfakes.FakeVolume
			)
			BeforeEach(func() {
				fakePipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)

				workerABaggageClaimClient = new(bcfakes.FakeClient)
				workerA.VolumeManagerReturns(workerABaggageClaimClient, true)
				gitVolume = new(bcfakes.FakeVolume)
				workerABaggageClaimClient.LookupVolumeReturns(gitVolume, true, nil)
			})

			It("keeps its cool", func() {
				err := baggageCollector.Collect()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBaggageCollectorDB.GetImageVolumeIdentifiersByBuildIDCallCount()).To(Equal(0))
			})
		})
	})
})
