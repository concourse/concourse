package lostandfound_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/lostandfoundfakes"
	"github.com/concourse/atc/resource"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc/db/dbfakes"
	wfakes "github.com/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volumes are reaped", func() {
	var (
		fakeWorkerClient *wfakes.FakeClient
		fakeWorker       *wfakes.FakeWorker
		fakeVolume       *wfakes.FakeVolume

		fakePipelineDBFactory          *dbfakes.FakePipelineDBFactory
		fakeBaggageCollectorDB         *lostandfoundfakes.FakeBaggageCollectorDB
		expectedOldResourceGracePeriod = 4 * time.Minute
		expectedOneOffTTL              = 5 * time.Hour

		baggageCollector          lostandfound.BaggageCollector
		returnedSavedVolume       db.SavedVolume
		newestReturnedSavedVolume db.SavedVolume
		returnedVolumes           []db.SavedVolume
	)

	BeforeEach(func() {
		fakeWorkerClient = new(wfakes.FakeClient)
		fakeWorker = new(wfakes.FakeWorker)
		fakeVolume = new(wfakes.FakeVolume)
		baggageCollectorLogger := lagertest.NewTestLogger("test")
		fakeBaggageCollectorDB = new(lostandfoundfakes.FakeBaggageCollectorDB)
		fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

		baggageCollector = lostandfound.NewBaggageCollector(
			baggageCollectorLogger,
			fakeWorkerClient,
			fakeBaggageCollectorDB,
			fakePipelineDBFactory,
			expectedOldResourceGracePeriod,
			expectedOneOffTTL,
		)

		returnedSavedVolume = db.SavedVolume{
			Volume: db.Volume{
				WorkerName: "a-new-worker",
				TTL:        time.Minute,
				Handle:     "some-handle",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash",
					},
				},
			},
			ID:        123,
			ExpiresIn: expectedOldResourceGracePeriod,
		}

		returnedVolumes = []db.SavedVolume{returnedSavedVolume}
	})

	JustBeforeEach(func() {
		fakeBaggageCollectorDB.GetVolumesReturns(returnedVolumes, nil)
	})

	Context("when the worker for a newest resource no longer exists", func() {
		var (
			fakeSavedPipeline          db.SavedPipeline
			fakePipelineDB             dbfakes.FakePipelineDB
			fakeSavedVersionedResource db.SavedVersionedResource
		)

		BeforeEach(func() {
			fakeSavedPipeline = db.SavedPipeline{
				Pipeline: db.Pipeline{
					Name: "some-pipeline",
					Config: atc.Config{
						Resources: atc.ResourceConfigs{
							atc.ResourceConfig{
								Name:   "our-resource",
								Type:   "git",
								Source: atc.Source{"some": "source"},
							},
						},
					},
					Version: 42,
				},
				ID:     7,
				Paused: false,
				TeamID: 13,
			}

			fakeSavedVersionedResource = db.SavedVersionedResource{
				ID:           123,
				Enabled:      true,
				ModifiedTime: time.Now(),
				VersionedResource: db.VersionedResource{
					Resource:   "our-resource",
					Type:       "git",
					Version:    db.Version{"some": "newest-version"},
					PipelineID: fakeSavedPipeline.ID,
				},
			}

			hashkey := resource.GenerateResourceHash(
				fakeSavedPipeline.Config.Resources[0].Source,
				fakeSavedPipeline.Config.Resources[0].Type,
			)
			newestReturnedSavedVolume = db.SavedVolume{
				Volume: db.Volume{
					WorkerName: "a-new-worker",
					TTL:        0,
					Handle:     "some-other-handle",
					Identifier: db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceVersion: atc.Version{"some": "newest-version"},
							ResourceHash:    hashkey,
						},
					},
				},
				ID:        124,
				ExpiresIn: 0,
			}

			returnedVolumes = append(returnedVolumes, newestReturnedSavedVolume)

			fakeBaggageCollectorDB.GetAllPipelinesReturns([]db.SavedPipeline{fakeSavedPipeline}, nil)
			fakePipelineDBFactory.BuildReturns(&fakePipelineDB)
			fakePipelineDB.GetLatestEnabledVersionedResourceReturns(fakeSavedVersionedResource, true, nil)
			fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
			fakeWorker.LookupVolumeReturns(fakeVolume, true, nil)
		})

		It("releases volume with final ttl", func() {
			err := baggageCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeVolume.ReleaseCallCount()).To(Equal(1))
		})
	})

	Context("when the worker can not be found", func() {
		BeforeEach(func() {
			fakeWorkerClient.GetWorkerReturns(nil, errors.New("no-worker-found"))
		})

		It("does not expire volume", func() {
			err := baggageCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBaggageCollectorDB.ReapVolumeCallCount()).To(Equal(0))
		})
	})

	Context("the volume is no longer found on the worker", func() {
		BeforeEach(func() {
			fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
			fakeWorker.LookupVolumeReturns(nil, false, nil)
		})

		It("removes the volume from the database", func() {
			err := baggageCollector.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeBaggageCollectorDB.ReapVolumeCallCount()).To(Equal(1))
			Expect(fakeBaggageCollectorDB.ReapVolumeArgsForCall(0)).To(Equal(returnedSavedVolume.Handle))
		})
	})
})
