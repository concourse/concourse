package lostandfound_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/lostandfound/fakes"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bcfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Baggage Collector", func() {

	var (
		fakeWorkerClient       *wfakes.FakeClient
		fakeWorker             *wfakes.FakeWorker
		fakeBaggageClaimClient *bcfakes.FakeClient

		fakeBaggageCollectorDB *fakes.FakeBaggageCollectorDB
		fakePipelineDBFactory  *dbfakes.FakePipelineDBFactory

		baggageCollector lostandfound.BaggageCollector
	)

	Context("when all the things return correctly", func() {
		type resourceConfigAndVersions struct {
			config            atc.ResourceConfig
			versions          []atc.Version
			versionsToDisable []int
		}

		type baggageCollectionExample struct {
			pipelineData map[string][]resourceConfigAndVersions
			volumeData   []db.Volume
			expectedTTLs map[string]time.Duration
		}

		DescribeTable("baggage collection",
			func(examples ...baggageCollectionExample) {
				var err error

				for _, example := range examples {
					fakeWorkerClient = new(wfakes.FakeClient)
					fakeWorker = new(wfakes.FakeWorker)
					fakeBaggageClaimClient = new(bcfakes.FakeClient)
					fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
					fakeWorker.VolumeManagerReturns(fakeBaggageClaimClient, true)
					baggageCollectorLogger := lagertest.NewTestLogger("test")

					fakeBaggageCollectorDB = new(fakes.FakeBaggageCollectorDB)
					fakePipelineDBFactory = new(dbfakes.FakePipelineDBFactory)

					baggageCollector = lostandfound.NewBaggageCollector(baggageCollectorLogger, fakeWorkerClient, fakeBaggageCollectorDB, fakePipelineDBFactory)

					var savedPipelines []db.SavedPipeline
					fakePipelineDBs := make(map[string]*dbfakes.FakePipelineDB)

					for name, data := range example.pipelineData {
						config := atc.Config{}

						for _, resourceData := range data {
							config.Resources = append(config.Resources, resourceData.config)
						}

						savedPipelines = append(savedPipelines, db.SavedPipeline{
							Pipeline: db.Pipeline{
								Name:   name,
								Config: config,
							},
						})

						fakePipelineDB := new(dbfakes.FakePipelineDB)

						fakePipelineDB.GetResourceHistoryMaxIDReturns(42, nil)

						resourceVersions := make(map[string][]*db.VersionHistory)
						for _, resourceInfo := range data {
							enabled := make([]bool, len(resourceInfo.versions))
							for i, _ := range enabled {
								enabled[i] = true
							}
							for _, i := range resourceInfo.versionsToDisable {
								enabled[i] = false
							}

							var history []*db.VersionHistory
							for i := len(resourceInfo.versions) - 1; i >= len(resourceInfo.versions)-5 && i >= 0; i-- {
								history = append(history, &db.VersionHistory{
									VersionedResource: db.SavedVersionedResource{
										Enabled: enabled[i],
										VersionedResource: db.VersionedResource{
											Version: db.Version(resourceInfo.versions[i]),
										},
									},
								})
							}
							resourceVersions[resourceInfo.config.Name] = history
						}
						fakePipelineDB.GetResourceHistoryCursorStub = func(resource string, startingID int, searchUpwards bool, numResults int) ([]*db.VersionHistory, bool, error) {
							Expect(startingID).To(Equal(42))
							Expect(searchUpwards).To(BeFalse())
							Expect(numResults).To(Equal(5))
							return resourceVersions[resource], false, nil
						}

						fakePipelineDBs[name] = fakePipelineDB
					}

					fakeBaggageCollectorDB.GetAllActivePipelinesReturns(savedPipelines, nil)

					fakePipelineDBFactory.BuildStub = func(savedPipeline db.SavedPipeline) db.PipelineDB {
						return fakePipelineDBs[savedPipeline.Name]
					}

					fakeVolumes := map[string]*bcfakes.FakeVolume{}

					var savedVolumes []db.SavedVolume
					for _, volume := range example.volumeData {
						savedVolumes = append(savedVolumes, db.SavedVolume{
							Volume: volume,
						})
						fakeVolumes[volume.Handle] = new(bcfakes.FakeVolume)
					}

					fakeBaggageCollectorDB.GetVolumesReturns(savedVolumes, nil)

					fakeBaggageClaimClient.LookupVolumeStub = func(_ lager.Logger, handle string) (baggageclaim.Volume, error) {
						vol, ok := fakeVolumes[handle]
						Expect(ok).To(BeTrue())
						return vol, nil
					}

					err = baggageCollector.Collect()
					Expect(err).NotTo(HaveOccurred())

					Expect(fakeBaggageClaimClient.LookupVolumeCallCount()).To(Equal(len(example.expectedTTLs)))
					var actualHandles []string
					for i := 0; i < fakeBaggageClaimClient.LookupVolumeCallCount(); i++ {
						_, actualHandle := fakeBaggageClaimClient.LookupVolumeArgsForCall(i)
						actualHandles = append(actualHandles, actualHandle)
					}

					var expectedHandles []string
					for handle, expectedTTL := range example.expectedTTLs {
						Expect(fakeVolumes[handle].ReleaseCallCount()).To(Equal(1))
						actualTTL := fakeVolumes[handle].ReleaseArgsForCall(0)
						Expect(actualTTL).To(Equal(expectedTTL))
						expectedHandles = append(expectedHandles, handle)
					}

					Expect(actualHandles).To(ConsistOf(expectedHandles))

					Expect(fakeBaggageCollectorDB.SetVolumeTTLCallCount()).To(Equal(len(example.expectedTTLs)))
					actualHandles = nil
					for i := 0; i < fakeBaggageCollectorDB.SetVolumeTTLCallCount(); i++ {
						actualSavedVolume, actualTTL := fakeBaggageCollectorDB.SetVolumeTTLArgsForCall(i)
						actualHandle := actualSavedVolume.Handle
						actualHandles = append(actualHandles, actualHandle)

						Expect(actualTTL).To(Equal(example.expectedTTLs[actualHandle]))
					}

					Expect(actualHandles).To(ConsistOf(expectedHandles))
				}
			},
			Entry("when there are volumes cached for multiple versions of the resource", baggageCollectionExample{
				pipelineData: map[string][]resourceConfigAndVersions{
					"pipeline-a": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "latest"},
							},
						},
						{
							config: atc.ResourceConfig{
								Name: "resource-b",
								Type: "some-b-type",
								Source: atc.Source{
									"some": "b-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "latest"},
							},
						},
					},
				},
				volumeData: []db.Volume{
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-1",
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-2",
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-3",
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-b-type{"some":"b-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-4",
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-b-type{"some":"b-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-1": 8 * time.Hour,
					"some-volume-handle-3": 8 * time.Hour,
				},
			}),
			Entry("it does not update ttls for the latest versions of a resource on each pipeline", baggageCollectionExample{
				pipelineData: map[string][]resourceConfigAndVersions{
					"pipeline-a": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "latest"},
							},
						},
					},
					"pipeline-b": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "latest"},
								{"version": "latest-in-b-but-not-yet-in-a"},
							},
						},
					},
				},
				volumeData: []db.Volume{
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-1",
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-2",
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-3",
						ResourceVersion: atc.Version{"version": "latest-in-b-but-not-yet-in-a"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-1": 8 * time.Hour,
				},
			}),
			Entry("it sets the ttls of disabled versions to soon", baggageCollectionExample{
				pipelineData: map[string][]resourceConfigAndVersions{
					"pipeline-a": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "older-disabled-version"},
								{"version": "latest"},
							},
							versionsToDisable: []int{1},
						},
					},
				},
				volumeData: []db.Volume{
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-2",
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-1",
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-3",
						ResourceVersion: atc.Version{"version": "older-disabled-version"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-1": 8 * time.Hour,
					"some-volume-handle-3": lostandfound.NoRelevantVersionsTTL,
				},
			}),
			Entry("it only updates ttls if they have a new value based on the rank", baggageCollectionExample{
				pipelineData: map[string][]resourceConfigAndVersions{
					"pipeline-a": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "1"},
								{"version": "2"},
								{"version": "3"},
								{"version": "4"},
								{"version": "5"},
								{"version": "6"},
								{"version": "7"},
							},
						},
					},
				},
				volumeData: []db.Volume{
					{
						WorkerName:      "some-worker",
						TTL:             10 * time.Minute,
						Handle:          "some-volume-handle-1",
						ResourceVersion: atc.Version{"version": "1"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-2",
						ResourceVersion: atc.Version{"version": "2"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             2 * time.Hour,
						Handle:          "some-volume-handle-3",
						ResourceVersion: atc.Version{"version": "3"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             4 * time.Hour,
						Handle:          "some-volume-handle-4",
						ResourceVersion: atc.Version{"version": "4"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             8 * time.Hour,
						Handle:          "some-volume-handle-5",
						ResourceVersion: atc.Version{"version": "5"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-6",
						ResourceVersion: atc.Version{"version": "6"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             8 * time.Hour,
						Handle:          "some-volume-handle-7",
						ResourceVersion: atc.Version{"version": "7"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-2": 10 * time.Minute,
					"some-volume-handle-3": 1 * time.Hour,
					"some-volume-handle-4": 2 * time.Hour,
					"some-volume-handle-5": 4 * time.Hour,
					"some-volume-handle-6": 8 * time.Hour,
					"some-volume-handle-7": 24 * time.Hour,
				},
			}),

			Entry("updates the db with the new ttl so that the next collection has the new ttl values", baggageCollectionExample{
				pipelineData: map[string][]resourceConfigAndVersions{
					"pipeline-a": []resourceConfigAndVersions{
						{
							config: atc.ResourceConfig{
								Name: "resource-a",
								Type: "some-a-type",
								Source: atc.Source{
									"some": "a-source",
								},
							},
							versions: []atc.Version{
								{"version": "older"},
								{"version": "latest"},
							},
						},
					},
				},
				volumeData: []db.Volume{
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-2",
						ResourceVersion: atc.Version{"version": "latest"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
						Handle:          "some-volume-handle-1",
						ResourceVersion: atc.Version{"version": "older"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-1": 8 * time.Hour,
				},
			}, baggageCollectionExample{}),
		)

	})

})
