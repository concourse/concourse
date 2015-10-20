package lostandfound_test

import (
	"database/sql"
	"encoding/json"
	"os"
	"time"

	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/lostandfound"
	"github.com/concourse/atc/postgresrunner"
	wfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/baggageclaim"
	bcfakes "github.com/concourse/baggageclaim/fakes"
)

var _ = Describe("Baggage Collector", func() {

	var (
		postgresRunner postgresrunner.Runner
		dbConn         *sql.DB
		dbProcess      ifrit.Process
		dbListener     *pq.Listener

		sqlDB             *db.SQLDB
		pipelineDBFactory db.PipelineDBFactory

		fakeWorkerClient       *wfakes.FakeClient
		fakeWorker             *wfakes.FakeWorker
		fakeBaggageClaimClient *bcfakes.FakeClient

		baggageCollector lostandfound.BaggageCollector
	)

	BeforeEach(func() {

		postgresRunner = postgresrunner.Runner{
			Port: 5432 + GinkgoParallelNode(),
		}

		dbProcess = ifrit.Invoke(postgresRunner)

		postgresRunner.CreateTestDB()

		dbLogger := lagertest.NewTestLogger("test")
		postgresRunner.Truncate()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(dbLogger, dbConn, bus)

		pipelineDBFactory = db.NewPipelineDBFactory(dbLogger, dbConn, bus, sqlDB)

	})

	AfterEach(func() {
		Expect(dbConn.Close()).To(Succeed())
		Expect(dbListener.Close()).To(Succeed())

		dbProcess.Signal(os.Interrupt)
		Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
	})

	Context("when all the things return correctly", func() {
		type resourceConfigAndVersions struct {
			config            atc.ResourceConfig
			versions          []atc.Version
			versionsToDisable []atc.Version
		}

		type baggageCollectionExample struct {
			pipelineData map[string][]resourceConfigAndVersions
			volumeData   []db.VolumeData
			expectedTTLs map[string]time.Duration
		}

		DescribeTable("baggage collection",
			func(examples ...baggageCollectionExample) {
				var err error

				fakeVolumes := map[string]*bcfakes.FakeVolume{}

				for _, example := range examples {
					fakeWorkerClient = new(wfakes.FakeClient)
					fakeWorker = new(wfakes.FakeWorker)
					fakeBaggageClaimClient = new(bcfakes.FakeClient)
					fakeWorkerClient.GetWorkerReturns(fakeWorker, nil)
					fakeWorker.VolumeManagerReturns(fakeBaggageClaimClient, true)
					baggageCollectorLogger := lagertest.NewTestLogger("test")

					baggageCollector = lostandfound.NewBaggageCollector(baggageCollectorLogger, fakeWorkerClient, sqlDB, pipelineDBFactory)

					for name, data := range example.pipelineData {
						var pipelineDB db.PipelineDB
						config := atc.Config{}

						for _, resourceData := range data {
							config.Resources = append(config.Resources, resourceData.config)

						}

						_, err = sqlDB.SaveConfig(name, config, db.ConfigVersion(1), db.PipelineUnpaused)
						Expect(err).NotTo(HaveOccurred())
						pipelineDB, err = pipelineDBFactory.BuildWithName(name)
						Expect(err).NotTo(HaveOccurred())

						for _, resourceData := range data {
							err = pipelineDB.SaveResourceVersions(resourceData.config, resourceData.versions)
							Expect(err).NotTo(HaveOccurred())

							for _, versionToDisable := range resourceData.versionsToDisable {
								var versionJSON []byte
								versionJSON, err = json.Marshal(versionToDisable)
								Expect(err).NotTo(HaveOccurred())
								_, err := dbConn.Exec(`
								update versioned_resources
								set enabled = false
								from versioned_resources vr
								inner join resources r on vr.resource_id = r.id
									AND r.name = $1
								inner join pipelines p on r.pipeline_id = p.id
									AND p.name = $2
								where vr.version = $3
									AND vr.id = versioned_resources.id
							`, resourceData.config.Name, name, versionJSON)
								Expect(err).NotTo(HaveOccurred())
							}
						}

					}

					for _, data := range example.volumeData {
						err := sqlDB.InsertVolumeData(data)
						Expect(err).NotTo(HaveOccurred())
						fakeVolumes[data.Handle] = new(bcfakes.FakeVolume)
					}

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
						actualTTL := fakeVolumes[handle].ReleaseArgsForCall(0)
						Expect(actualTTL).To(Equal(expectedTTL))
						expectedHandles = append(expectedHandles, handle)
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
				volumeData: []db.VolumeData{
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
				volumeData: []db.VolumeData{
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
							versionsToDisable: []atc.Version{
								{"version": "older-disabled-version"},
							},
						},
					},
				},
				volumeData: []db.VolumeData{
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
				volumeData: []db.VolumeData{
					{
						WorkerName:      "some-worker",
						TTL:             24 * time.Hour,
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
					"some-volume-handle-1": 10 * time.Minute,
					"some-volume-handle-2": 10 * time.Minute,
					"some-volume-handle-3": 1 * time.Hour,
					"some-volume-handle-4": 2 * time.Hour,
					"some-volume-handle-5": 4 * time.Hour,
					"some-volume-handle-6": 8 * time.Hour,
					"some-volume-handle-7": 24 * time.Hour,
				},
			}),

			Entry("it deletes expired volumes as the first step of lookup", baggageCollectionExample{
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
								{"version": "expired"},
								{"version": "older"},
								{"version": "latest"},
							},
							versionsToDisable: []atc.Version{
								{"version": "older-disabled-version"},
							},
						},
					},
				},
				volumeData: []db.VolumeData{
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
						TTL:             -time.Hour,
						Handle:          "some-volume-handle-3",
						ResourceVersion: atc.Version{"version": "expired"},
						ResourceHash:    `some-a-type{"some":"a-source"}`,
					},
				},
				expectedTTLs: map[string]time.Duration{
					"some-volume-handle-1": 8 * time.Hour,
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
				volumeData: []db.VolumeData{
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
