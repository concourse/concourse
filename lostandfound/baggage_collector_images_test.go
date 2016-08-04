package lostandfound_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/dbfakes"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/lostandfoundfakes"
	"github.com/concourse/atc/worker"
	wfakes "github.com/concourse/atc/worker/workerfakes"
)

var _ = Describe("Baggage-collecting image resource volumes", func() {
	Context("when there is a single job", func() {
		var (
			fakeWorkerClient *wfakes.FakeClient
			workerA          *wfakes.FakeWorker

			workerB      *wfakes.FakeWorker
			dockerVolume *wfakes.FakeVolume

			workerC            *wfakes.FakeWorker
			crossedWiresVolume *wfakes.FakeVolume

			fakeBaggageCollectorDB *lostandfoundfakes.FakeBaggageCollectorDB
			fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory
			fakeBuild2             *dbfakes.FakeBuild
			fakeBuild3             *dbfakes.FakeBuild

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
			dockerVolume = new(wfakes.FakeVolume)
			workerB.LookupVolumeReturns(dockerVolume, true, nil)

			workerC = new(wfakes.FakeWorker)
			crossedWiresVolume = new(wfakes.FakeVolume)
			workerC.LookupVolumeReturns(crossedWiresVolume, true, nil)

			workerMap := map[string]*wfakes.FakeWorker{
				"workerA": workerA,
				"workerB": workerB,
				"workerC": workerC,
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

			fakeBuild2 = new(dbfakes.FakeBuild)
			fakeBuild3 = new(dbfakes.FakeBuild)
			fakeBuild2.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{
				{
					ResourceVersion: atc.Version{"ref": "rence"},
					ResourceHash:    "git:zxcvbnm",
				},
				{
					ResourceVersion: atc.Version{"digest": "readers"},
					ResourceHash:    "docker:qwertyuiop",
				},
			}, nil)
			fakeBuild3.GetImageResourceCacheIdentifiersReturns([]db.ResourceCacheIdentifier{
				{
					ResourceVersion: atc.Version{"ref": "rence"},
					ResourceHash:    "docker:qwertyuiop",
				},
			}, nil)

			fakePipelineDB = new(dbfakes.FakePipelineDB)

			fakePipelineDB.GetJobFinishedAndNextBuildReturns(fakeBuild2, fakeBuild3, nil)

			fakePipelineDBFactory.BuildReturns(fakePipelineDB)

			savedVolumes := []db.SavedVolume{
				{
					Volume: db.Volume{
						WorkerName: "workerA",
						TTL:        expectedLatestVersionTTL,
						Handle:     "git-volume-handle",
						Identifier: db.VolumeIdentifier{
							ResourceCache: &db.ResourceCacheIdentifier{
								ResourceVersion: atc.Version{"ref": "rence"},
								ResourceHash:    "git:zxcvbnm",
							},
						},
					},
				},
				{
					Volume: db.Volume{
						WorkerName: "workerB",
						TTL:        expectedOldVersionTTL,
						Handle:     "docker-volume-handle",
						Identifier: db.VolumeIdentifier{
							ResourceCache: &db.ResourceCacheIdentifier{
								ResourceVersion: atc.Version{"digest": "readers"},
								ResourceHash:    "docker:qwertyuiop",
							},
						},
					},
				},
				{
					Volume: db.Volume{
						WorkerName: "workerC",
						TTL:        92 * time.Minute,
						Handle:     "crossed-wires-volume-handle",
						Identifier: db.VolumeIdentifier{
							ResourceCache: &db.ResourceCacheIdentifier{
								ResourceVersion: atc.Version{"ref": "rence"},
								ResourceHash:    "docker:qwertyuiop",
							},
						},
					},
				},
			}
			fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)
		})

		It("preserves only the image versions used by the latest finished build of each job", func() {
			err := baggageCollector.Run()
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeBaggageCollectorDB.GetAllPipelinesCallCount()).To(Equal(1))
			Expect(fakePipelineDBFactory.BuildCallCount()).To(Equal(1))
			Expect(fakePipelineDBFactory.BuildArgsForCall(0)).To(Equal(savedPipeline))
			Expect(fakePipelineDB.GetJobFinishedAndNextBuildCallCount()).To(Equal(1))
			Expect(fakePipelineDB.GetJobFinishedAndNextBuildArgsForCall(0)).To(Equal("my-precious-job"))
			Expect(fakeBuild2.GetImageResourceCacheIdentifiersCallCount()).To(Equal(1))
			Expect(fakeBaggageCollectorDB.GetVolumesCallCount()).To(Equal(1))
			Expect(fakeWorkerClient.GetWorkerCallCount()).To(Equal(3))

			var handle string
			Expect(workerB.LookupVolumeCallCount()).To(Equal(1))
			_, handle = workerB.LookupVolumeArgsForCall(0)
			Expect(handle).To(Equal("docker-volume-handle"))
			Expect(dockerVolume.ReleaseCallCount()).To(Equal(1))
			Expect(dockerVolume.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedLatestVersionTTL)))

			Expect(workerC.LookupVolumeCallCount()).To(Equal(1))
			_, handle = workerC.LookupVolumeArgsForCall(0)
			Expect(handle).To(Equal("crossed-wires-volume-handle"))
			Expect(crossedWiresVolume.ReleaseCallCount()).To(Equal(1))
			Expect(crossedWiresVolume.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedOldVersionTTL)))
		})

		Context("When the job has no previously finished builds", func() {
			var (
				gitVolume *wfakes.FakeVolume
			)
			BeforeEach(func() {
				fakePipelineDB.GetJobFinishedAndNextBuildReturns(nil, nil, nil)

				gitVolume = new(wfakes.FakeVolume)
				workerA.LookupVolumeReturns(gitVolume, true, nil)
			})

			It("keeps its cool", func() {
				err := baggageCollector.Run()
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeBuild2.GetImageResourceCacheIdentifiersCallCount()).To(Equal(0))
			})
		})
	})
	Context("when multiple jobs get the same image resource", func() {
		var (
			fakeWorkerClient *wfakes.FakeClient

			workerA  *wfakes.FakeWorker
			volumeA1 *wfakes.FakeVolume
			volumeA2 *wfakes.FakeVolume

			workerB  *wfakes.FakeWorker
			volumeB1 *wfakes.FakeVolume

			fakeBaggageCollectorDB *lostandfoundfakes.FakeBaggageCollectorDB
			fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory
			fakeBuild              *dbfakes.FakeBuild

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
			volumeA1 = new(wfakes.FakeVolume)
			volumeA2 = new(wfakes.FakeVolume)
			workerA.LookupVolumeStub = func(logger lager.Logger, handle string) (worker.Volume, bool, error) {
				switch handle {
				case "volume-a1":
					return volumeA1, true, nil
				case "volume-a2":
					return volumeA2, true, nil
				default:
					panic("unknown volume handle")
				}
			}

			workerB = new(wfakes.FakeWorker)
			volumeB1 = new(wfakes.FakeVolume)
			workerB.LookupVolumeReturns(volumeB1, true, nil)

			fakeWorkerClient.GetWorkerStub = func(name string) (worker.Worker, error) {
				switch name {
				case "worker-a":
					return workerA, nil
				case "worker-b":
					return workerB, nil
				default:
					panic("unknown worker name")
				}
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
								Name: "job-a1",
							},
							{
								Name: "job-a2",
							},
							{
								Name: "job-b1",
							},
						},
					},
				},
			}
			fakeBaggageCollectorDB.GetAllPipelinesReturns([]db.SavedPipeline{savedPipeline}, nil)

			fakeBuild = new(dbfakes.FakeBuild)
			fakeBuild.GetImageResourceCacheIdentifiersReturns(
				[]db.ResourceCacheIdentifier{
					{
						ResourceVersion: atc.Version{"ref": "rence"},
						ResourceHash:    "git:zxcvbnm",
					},
				},
				nil,
			)

			fakePipelineDB = new(dbfakes.FakePipelineDB)
			fakePipelineDB.GetJobFinishedAndNextBuildStub = func(jobName string) (db.Build, db.Build, error) {
				switch jobName {
				case "job-a1":
					return fakeBuild, nil, nil
				case "job-a2":
					return fakeBuild, nil, nil
				case "job-b1":
					return fakeBuild, nil, nil
				default:
					panic("unknown job name")
				}
			}
			fakePipelineDBFactory.BuildReturns(fakePipelineDB)
		})

		DescribeTable("It preserves a single volume per worker corresponding to that image resource",
			func(savedVolumes []db.SavedVolume) {
				fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)

				err := baggageCollector.Run()
				Expect(err).NotTo(HaveOccurred())

				Expect(volumeA1.ReleaseCallCount()).To(Equal(1))
				Expect(volumeA1.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedLatestVersionTTL)))

				Expect(volumeA2.ReleaseCallCount()).To(Equal(1))
				Expect(volumeA2.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedOldVersionTTL)))

				Expect(volumeB1.ReleaseCallCount()).To(Equal(1))
				Expect(volumeB1.ReleaseArgsForCall(0)).To(Equal(worker.FinalTTL(expectedLatestVersionTTL)))
			},
			Entry("and it chooses the ones with the first handle in alphabetical order",
				[]db.SavedVolume{
					{
						Volume: db.Volume{
							WorkerName: "worker-a",
							TTL:        expectedOldVersionTTL,
							Handle:     "volume-a1",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
					{
						Volume: db.Volume{
							WorkerName: "worker-a",
							TTL:        expectedLatestVersionTTL,
							Handle:     "volume-a2",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
					{
						Volume: db.Volume{
							WorkerName: "worker-b",
							TTL:        expectedOldVersionTTL,
							Handle:     "volume-b1",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
				},
			),
			Entry("and it chooses the ones with the first handle in alphabetical order",
				[]db.SavedVolume{
					{
						Volume: db.Volume{
							WorkerName: "worker-a",
							TTL:        expectedLatestVersionTTL,
							Handle:     "volume-a2",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
					{
						Volume: db.Volume{
							WorkerName: "worker-b",
							TTL:        expectedOldVersionTTL,
							Handle:     "volume-b1",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
					{
						Volume: db.Volume{
							WorkerName: "worker-a",
							TTL:        expectedOldVersionTTL,
							Handle:     "volume-a1",
							Identifier: db.VolumeIdentifier{
								ResourceCache: &db.ResourceCacheIdentifier{
									ResourceVersion: atc.Version{"ref": "rence"},
									ResourceHash:    "git:zxcvbnm",
								},
							},
						},
					},
				},
			),
		)
	})
})
