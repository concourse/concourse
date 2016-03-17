package db_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
)

var _ = Describe("Keeping track of volumes", func() {
	var dbConn db.Conn
	var listener *pq.Listener

	var database db.DB
	var pipelineDB db.PipelineDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB := db.NewSQL(dbConn, bus)
		database = sqlDB

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, sqlDB)
		_, err := database.SaveTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		config := atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
			},
		}
		sqlDB.SaveConfig("some-team", "some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
		pipelineDB, err = pipelineDBFactory.BuildWithTeamNameAndName("some-team", "some-pipeline")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("volume data", func() {
		var (
			volumeToInsert  db.Volume
			workerToInsert  db.WorkerInfo
			workerToInsert2 db.WorkerInfo
			insertedWorker  db.SavedWorker
			insertedWorker2 db.SavedWorker
		)

		BeforeEach(func() {
			volumeToInsert = db.Volume{
				TTL:    time.Hour,
				Handle: "some-volume-handle",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"some": "version"},
					ResourceHash:    "some-hash",
				},
			}

			workerToInsert = db.WorkerInfo{
				GardenAddr:       "some-garden-address",
				BaggageclaimURL:  "some-baggageclaim-url",
				ActiveContainers: 0,
				ResourceTypes:    []atc.WorkerResourceType{},
				Platform:         "linux",
				Tags:             []string{"vsphere"},
				Name:             "some-worker",
			}
			workerToInsert2 = db.WorkerInfo{
				GardenAddr:       "second-garden-address",
				BaggageclaimURL:  "some-baggageclaim-url",
				ActiveContainers: 0,
				ResourceTypes:    []atc.WorkerResourceType{},
				Platform:         "linux",
				Tags:             []string{"vsphere"},
				Name:             "second-worker",
			}
		})

		Describe("output volumes", func() {
			JustBeforeEach(func() {
				var err error

				insertedWorker, err = database.SaveWorker(workerToInsert, 2*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				insertedWorker2, err = database.SaveWorker(workerToInsert2, 2*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				volumeToInsert.WorkerName = insertedWorker.Name
				err = database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))
				actualVolume := volumes[0]
				Expect(actualVolume.WorkerName).To(Equal(volumeToInsert.WorkerName))
				Expect(actualVolume.TTL).To(Equal(volumeToInsert.TTL))
				Expect(actualVolume.ExpiresIn).To(BeNumerically("~", volumeToInsert.TTL, time.Second))
				Expect(actualVolume.Handle).To(Equal(volumeToInsert.Handle))
				Expect(actualVolume.ResourceVersion).To(Equal(volumeToInsert.ResourceVersion))
				Expect(actualVolume.ResourceHash).To(Equal(volumeToInsert.ResourceHash))
				Expect(actualVolume.WorkerName).To(Equal(insertedWorker.Name))
				Expect(actualVolume.OriginalVolumeHandle).To(BeEmpty())
			})

			Describe("cow volumes", func() {
				var (
					originalVolume          db.SavedVolume
					cowVolumeToInsertHandle string
					ttl                     time.Duration
				)

				JustBeforeEach(func() {
					cowVolumeToInsertHandle = "cow-volume-handle"
					ttl = 5 * time.Minute

					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(1))
					originalVolume = volumes[0]

					err = database.InsertCOWVolume(originalVolume.Handle, cowVolumeToInsertHandle, ttl)
					Expect(err).NotTo(HaveOccurred())
				})

				It("can be retrieved", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(2))
					cowVolume := volumes[1]
					Expect(cowVolume.Handle).To(Equal(cowVolumeToInsertHandle))
					Expect(cowVolume.OriginalVolumeHandle).To(Equal(originalVolume.Handle))
					Expect(cowVolume.TTL).To(Equal(ttl))
					Expect(cowVolume.ExpiresIn).To(BeNumerically("~", ttl, time.Second))

					Expect(cowVolume.WorkerName).To(Equal(originalVolume.WorkerName))
					Expect(cowVolume.WorkerName).To(Equal(insertedWorker.Name))
					Expect(cowVolume.ResourceVersion).To(BeNil())
					Expect(cowVolume.ResourceHash).To(Equal(""))
				})
			})

			It("can be reaped", func() {
				volumeToInsert2 := db.Volume{
					WorkerName: insertedWorker2.Name,
					TTL:        time.Hour,
					Handle:     "some-volume-handle2",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash2",
					},
				}
				err := database.InsertVolume(volumeToInsert2)
				Expect(err).NotTo(HaveOccurred())

				workerToInsert3 := db.WorkerInfo{
					GardenAddr:       "third-garden-address",
					BaggageclaimURL:  "some-baggageclaim-url",
					ActiveContainers: 0,
					ResourceTypes:    []atc.WorkerResourceType{},
					Platform:         "linux",
					Tags:             []string{"vsphere"},
					Name:             "third-worker",
				}
				insertedWorker3, err := database.SaveWorker(workerToInsert3, 2*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				volumeToInsert3 := db.Volume{
					WorkerName: insertedWorker3.Name,
					TTL:        time.Hour,
					Handle:     "some-volume-handle3",
					VolumeIdentifier: db.VolumeIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash3",
					},
				}
				err = database.InsertVolume(volumeToInsert3)
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(3))

				reapedVolume := volumes[0]
				err = database.ReapVolume(reapedVolume.Handle)
				Expect(err).NotTo(HaveOccurred())

				volumes, err = database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(2))
				Expect(volumes).NotTo(ContainElement(reapedVolume))
			})

			It("can insert the same data twice, without erroring or data duplication", func() {
				err := database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))
			})

			It("can create the same volume on a different worker", func() {
				volumeToInsert.WorkerName = insertedWorker2.Name
				err := database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(2))
			})

			Context("expired volumes", func() {
				BeforeEach(func() {
					volumeToInsert.TTL = -time.Hour
				})

				It("does not return them", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(0))
				})
			})

			Context("TTL's", func() {
				It("can be retrieved by volume handler", func() {
					actualTTL, err := database.GetVolumeTTL(volumeToInsert.Handle)
					Expect(err).NotTo(HaveOccurred())
					Expect(actualTTL).To(Equal(volumeToInsert.TTL))
				})

				It("can be updated", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(1))

					err = database.SetVolumeTTL(volumes[0].Handle, -time.Hour)
					Expect(err).NotTo(HaveOccurred())

					volumes, err = database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(Equal(0))
				})

				It("can be updated to zero to mean 'keep around forever'", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(1))

					err = database.SetVolumeTTL(volumes[0].Handle, 0)
					Expect(err).NotTo(HaveOccurred())

					volumes, err = database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(1))

					Expect(volumes[0].TTL).To(BeZero())
					Expect(volumes[0].ExpiresIn).To(BeZero())
				})

				Context("when the ttl is set to 0", func() {
					BeforeEach(func() {
						volumeToInsert.TTL = 0
					})

					It("sets the expiration to null", func() {
						volumes, err := database.GetVolumes()
						Expect(err).NotTo(HaveOccurred())
						Expect(volumes).To(HaveLen(1))
						Expect(volumes[0].TTL).To(Equal(time.Duration(0)))
						Expect(volumes[0].ExpiresIn).To(Equal(time.Duration(0)))
					})
				})
			})
		})

		Describe("output volumes", func() {
			var outputVolume db.Volume

			BeforeEach(func() {
				outputVolume = db.Volume{
					WorkerName: insertedWorker.Name,
					TTL:        5 * time.Minute,
					Handle:     "my-output-handle",
				}

				err := database.InsertOutputVolume(outputVolume)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				volumes, err := database.GetVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))
				savedOutputVolume := volumes[0]
				Expect(savedOutputVolume.WorkerName).To(Equal(outputVolume.WorkerName))
				Expect(savedOutputVolume.TTL).To(Equal(outputVolume.TTL))
				Expect(savedOutputVolume.Handle).To(Equal(outputVolume.Handle))
				Expect(savedOutputVolume.OriginalVolumeHandle).To(BeEmpty())
				Expect(savedOutputVolume.ExpiresIn).To(BeNumerically("~", outputVolume.TTL, time.Second))

				Expect(savedOutputVolume.ResourceVersion).To(BeNil())
				Expect(savedOutputVolume.ResourceHash).To(BeEmpty())
			})
		})
	})

	Describe("GetVolumesForOneOffBuildImageResources", func() {
		It("returns all volumes containing image resource versions which were used in one-off builds", func() {
			oneOffBuildA, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			oneOffBuildB, err := database.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			jobBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			// To show that it returns volumes that are used in both one-off builds and job builds
			volume1 := db.Volume{
				WorkerName: "worker-1",
				TTL:        2 * time.Minute,
				Handle:     "volume-1",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-1"},
					ResourceHash:    `docker:{"repository":"repository-1"}`,
				},
			}
			err = database.InsertVolume(volume1)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(oneOffBuildA.ID, "plan-id-1", volume1.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(jobBuild.ID, "plan-id-1", volume1.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			// To show that it can return more than one volume per build ID
			volume2 := db.Volume{
				WorkerName: "worker-2",
				TTL:        2 * time.Minute,
				Handle:     "volume-2",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-2"},
					ResourceHash:    `docker:{"repository":"repository-2"}`,
				},
			}
			err = database.InsertVolume(volume2)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(oneOffBuildA.ID, "plan-id-2", volume2.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			// To show that it can return more than one volume per VolumeIdentifier
			volume3 := db.Volume{
				WorkerName: "worker-3",
				TTL:        2 * time.Minute,
				Handle:     "volume-3",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-1"},
					ResourceHash:    `docker:{"repository":"repository-1"}`,
				},
			}
			err = database.InsertVolume(volume3)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(oneOffBuildA.ID, "plan-id-3", volume3.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			// To show that it can return volumes from multiple one-off builds
			volume4 := db.Volume{
				WorkerName: "worker-4",
				TTL:        2 * time.Minute,
				Handle:     "volume-4",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-4"},
					ResourceHash:    `docker:{"repository":"repository-4"}`,
				},
			}
			err = database.InsertVolume(volume4)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(oneOffBuildB.ID, "plan-id-4", volume4.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			// To show that it ignores volumes from job builds even if part of the VolumeIdentifier matches
			volume5 := db.Volume{
				WorkerName: "worker-5",
				TTL:        2 * time.Minute,
				Handle:     "volume-5",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-1"},
					ResourceHash:    `docker:{"repository":"repository-2"}`,
				},
			}
			err = database.InsertVolume(volume5)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(jobBuild.ID, "plan-id-5", volume5.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			// To show that it reaps expired volumes
			volume6 := db.Volume{
				WorkerName: "worker-6",
				TTL:        -time.Hour,
				Handle:     "volume-6",
				VolumeIdentifier: db.VolumeIdentifier{
					ResourceVersion: atc.Version{"digest": "digest-6"},
					ResourceHash:    `docker:{"repository":"repository-6"}`,
				},
			}
			err = database.InsertVolume(volume6)
			Expect(err).NotTo(HaveOccurred())
			err = database.SaveImageResourceVersion(oneOffBuildA.ID, "plan-id-6", volume6.VolumeIdentifier)
			Expect(err).NotTo(HaveOccurred())

			actualSavedVolumes, err := database.GetVolumesForOneOffBuildImageResources()
			Expect(err).NotTo(HaveOccurred())
			var actualVolumes []db.Volume
			for _, actualSavedVolume := range actualSavedVolumes {
				actualVolumes = append(actualVolumes, actualSavedVolume.Volume)
			}
			Expect(actualVolumes).To(ConsistOf(volume1, volume2, volume3, volume4))
		})
	})
})
