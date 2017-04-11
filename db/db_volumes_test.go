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
	var teamDB db.TeamDB
	var teamID int

	var workerToInsert db.WorkerInfo
	var workerToInsert2 db.WorkerInfo
	var insertedWorker db.SavedWorker
	var insertedWorker2 db.SavedWorker

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = db.Wrap(postgresRunner.Open())
		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)

		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		lockFactory := db.NewLockFactory(postgresRunner.OpenSingleton())
		sqlDB := db.NewSQL(dbConn, bus, lockFactory)
		database = sqlDB

		pipelineDBFactory := db.NewPipelineDBFactory(dbConn, bus, lockFactory)
		team, err := database.CreateTeam(db.Team{Name: "some-team"})
		Expect(err).NotTo(HaveOccurred())
		teamID = team.ID

		config := atc.Config{
			Jobs: atc.JobConfigs{
				{
					Name: "some-job",
				},
			},
		}
		teamDBFactory := db.NewTeamDBFactory(dbConn, bus, lockFactory)
		teamDB = teamDBFactory.GetTeamDB("some-team")
		savedPipeline, _, err := teamDB.SaveConfigToBeDeprecated("some-pipeline", config, db.ConfigVersion(1), db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB = pipelineDBFactory.Build(savedPipeline)

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

		insertedWorker, err = database.SaveWorker(workerToInsert, 2*time.Minute)
		Expect(err).NotTo(HaveOccurred())

		insertedWorker2, err = database.SaveWorker(workerToInsert2, 2*time.Minute)
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
			volumeToInsert db.Volume
		)

		BeforeEach(func() {
			volumeToInsert = db.Volume{
				TTL:    time.Hour,
				Handle: "some-volume-handle",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash",
					},
				},
				WorkerName: "some-worker",
			}
		})

		Describe("GetVolumesByIdentifier", func() {
			var identifier db.VolumeIdentifier

			BeforeEach(func() {
				identifier = db.VolumeIdentifier{
					COW: &db.COWIdentifier{
						ParentVolumeHandle: "parent-volume-handle",
					},
				}

				err := database.InsertVolume(db.Volume{
					Handle:      "volume-1-handle",
					TeamID:      teamID,
					WorkerName:  "some-worker",
					TTL:         5 * time.Minute,
					Identifier:  identifier,
					SizeInBytes: int64(1),
				})
				Expect(err).NotTo(HaveOccurred())

				err = database.InsertVolume(db.Volume{
					Handle:      "volume-2-handle",
					WorkerName:  "some-worker",
					TTL:         5 * time.Minute,
					Identifier:  identifier,
					SizeInBytes: int64(1),
				})
				Expect(err).NotTo(HaveOccurred())

				err = database.InsertVolume(db.Volume{
					Handle:      "volume-3-handle",
					WorkerName:  "some-worker",
					TTL:         5 * time.Minute,
					Identifier:  identifier,
					SizeInBytes: int64(1),
				})
				Expect(err).NotTo(HaveOccurred())

				result, err := dbConn.Exec("UPDATE volumes SET id = 11 WHERE handle = 'volume-2-handle'")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
				result, err = dbConn.Exec("UPDATE volumes SET id = 12 WHERE handle = 'volume-1-handle'")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
				result, err = dbConn.Exec("UPDATE volumes SET id = 13 WHERE handle = 'volume-3-handle'")
				Expect(err).NotTo(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("returns volumes sorted by ID", func() {
				volumes, err := database.GetVolumesByIdentifier(identifier)
				Expect(err).NotTo(HaveOccurred())
				for i, volume := range volumes {
					switch i {
					case 0:
						Expect(volume.Handle).To(Equal("volume-2-handle"))
						Expect(volume.TeamID).To(BeZero())
					case 1:
						Expect(volume.Handle).To(Equal("volume-1-handle"))
						Expect(volume.TeamID).To(Equal(teamID))
					case 2:
						Expect(volume.Handle).To(Equal("volume-3-handle"))
						Expect(volume.TeamID).To(BeZero())
					}
				}
			})

			It("does not return expired volumes", func() {
				err := database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumesByIdentifier(volumeToInsert.Identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))
				Expect(volumes[0].Handle).To(Equal("some-volume-handle"))

				err = database.SetVolumeTTL("some-volume-handle", -time.Minute)
				Expect(err).NotTo(HaveOccurred())

				volumes, err = database.GetVolumesByIdentifier(volumeToInsert.Identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(BeEmpty())
			})
		})

		Describe("SetVolumeTTLAndSizeInBytes", func() {
			var identifier db.VolumeIdentifier

			BeforeEach(func() {
				identifier = db.VolumeIdentifier{
					COW: &db.COWIdentifier{
						ParentVolumeHandle: "parent-volume-handle",
					},
				}

				err := database.InsertVolume(db.Volume{
					Handle:      "volume-1-handle",
					WorkerName:  "some-worker",
					TTL:         5 * time.Minute,
					Identifier:  identifier,
					SizeInBytes: int64(1),
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets volume size", func() {
				err := database.SetVolumeTTLAndSizeInBytes("volume-1-handle", 0, int64(1024))
				Expect(err).NotTo(HaveOccurred())
				volumes, err := database.GetVolumesByIdentifier(identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))
				Expect(volumes[0].SizeInBytes).To(Equal(int64(1024)))
			})

			It("can set the volume size to 5000000000", func() {
				err := database.SetVolumeTTLAndSizeInBytes("volume-1-handle", 0, int64(5000000000))
				Expect(err).NotTo(HaveOccurred())
				volumes, err := database.GetVolumesByIdentifier(identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))
				Expect(volumes[0].SizeInBytes).To(Equal(int64(5000000000)))
			})

			It("sets volume size", func() {
				err := database.SetVolumeTTLAndSizeInBytes("volume-1-handle", 10*time.Minute, int64(1024))
				Expect(err).NotTo(HaveOccurred())
				volumes, err := database.GetVolumesByIdentifier(identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))
				Expect(volumes[0].TTL).To(Equal(10 * time.Minute))
				Expect(volumes[0].ExpiresIn).NotTo(BeZero())
			})

			It("can be updated to zero to mean 'keep around forever'", func() {
				err := database.SetVolumeTTLAndSizeInBytes("volume-1-handle", 0, int64(1024))
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumesByIdentifier(identifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(volumes).To(HaveLen(1))

				Expect(volumes[0].TTL).To(BeZero())
				Expect(volumes[0].ExpiresIn).To(BeZero())
			})
		})

		Describe("cow volumes", func() {
			var cowIdentifier db.VolumeIdentifier

			JustBeforeEach(func() {
				cowIdentifier = db.VolumeIdentifier{
					COW: &db.COWIdentifier{
						ParentVolumeHandle: "parent-volume-handle",
					},
				}

				err := database.InsertVolume(db.Volume{
					Handle:     "cow-volume-handle",
					WorkerName: "some-worker",
					TTL:        5 * time.Minute,
					Identifier: cowIdentifier,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				cowVolumes, err := database.GetVolumesByIdentifier(cowIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(cowVolumes)).To(Equal(1))
				cowVolume := cowVolumes[0]
				Expect(cowVolume.Handle).To(Equal("cow-volume-handle"))
				Expect(cowVolume.Volume.Identifier.COW.ParentVolumeHandle).To(Equal("parent-volume-handle"))
				Expect(cowVolume.TTL).To(Equal(5 * time.Minute))
				Expect(cowVolume.ExpiresIn).To(BeNumerically("~", 5*time.Minute, time.Second))
				Expect(cowVolume.WorkerName).To(Equal("some-worker"))
			})
		})

		Describe("resource cache", func() {
			JustBeforeEach(func() {
				var err error

				volumeToInsert.WorkerName = insertedWorker.Name
				err = database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				actualVolumes, err := database.GetVolumesByIdentifier(
					db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceVersion: atc.Version{"some": "version"},
							ResourceHash:    "some-hash",
						},
					})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(actualVolumes)).To(Equal(1))
				actualVolume := actualVolumes[0]
				Expect(actualVolume.WorkerName).To(Equal(volumeToInsert.WorkerName))
				Expect(actualVolume.TTL).To(Equal(volumeToInsert.TTL))
				Expect(actualVolume.ExpiresIn).To(BeNumerically("~", volumeToInsert.TTL, time.Second))
				Expect(actualVolume.Handle).To(Equal(volumeToInsert.Handle))
				Expect(actualVolume.Volume.Identifier).To(Equal(volumeToInsert.Identifier))
				Expect(actualVolume.WorkerName).To(Equal(insertedWorker.Name))
			})

			It("can be reaped", func() {
				err := database.InsertVolume(volumeToInsert)
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumesByIdentifier(db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash",
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(Equal(1))

				err = database.ReapVolume(volumes[0].Handle)
				Expect(err).NotTo(HaveOccurred())

				volumes, err = database.GetVolumesByIdentifier(db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"some": "version"},
						ResourceHash:    "some-hash",
					},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(volumes)).To(BeZero())
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

				It("does not return them from GetVolumes", func() {
					volumes, err := database.GetVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(volumes).To(HaveLen(0))
				})

				It("does not return it from GetVolumesByIdentifier", func() {
					volumes, err := database.GetVolumesByIdentifier(db.VolumeIdentifier{
						ResourceCache: &db.ResourceCacheIdentifier{
							ResourceVersion: atc.Version{"some": "version"},
							ResourceHash:    "some-hash",
						},
					})
					Expect(err).NotTo(HaveOccurred())
					Expect(len(volumes)).To(BeZero())
				})
			})

			Context("TTL's", func() {
				It("can be retrieved by volume handler", func() {
					actualTTL, found, err := database.GetVolumeTTL(volumeToInsert.Handle)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(actualTTL).To(Equal(volumeToInsert.TTL))
				})

				It("returns false if the volume doesn't exist", func() {
					actualTTL, found, err := database.GetVolumeTTL("bogus-handle")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(actualTTL).To(BeZero())
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

		It("does not return an error if no volume exists with that identifier", func() {
			_, err := database.GetVolumesByIdentifier(db.VolumeIdentifier{
				Output: &db.OutputIdentifier{
					Name: "some-output",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("output volumes", func() {
			var outputVolume db.Volume
			var outputIdentifier db.VolumeIdentifier

			BeforeEach(func() {
				outputIdentifier = db.VolumeIdentifier{
					Output: &db.OutputIdentifier{
						Name: "some-output",
					},
				}
				outputVolume = db.Volume{
					WorkerName: insertedWorker.Name,
					TTL:        5 * time.Minute,
					Handle:     "my-output-handle",
					Identifier: outputIdentifier,
				}

				err := database.InsertVolume(outputVolume)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				savedOutputVolumes, err := database.GetVolumesByIdentifier(outputIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(savedOutputVolumes)).To(Equal(1))

				savedOutputVolume := savedOutputVolumes[0]
				Expect(savedOutputVolume.WorkerName).To(Equal(outputVolume.WorkerName))
				Expect(savedOutputVolume.TTL).To(Equal(outputVolume.TTL))
				Expect(savedOutputVolume.Handle).To(Equal(outputVolume.Handle))
				Expect(savedOutputVolume.Volume.Identifier.Output.Name).To(Equal("some-output"))
				Expect(savedOutputVolume.ExpiresIn).To(BeNumerically("~", outputVolume.TTL, time.Second))
			})
		})

		Describe("replication volumes", func() {
			var replicationVolume db.Volume
			var replicationIdentifier db.VolumeIdentifier

			BeforeEach(func() {
				replicationIdentifier = db.VolumeIdentifier{
					Replication: &db.ReplicationIdentifier{
						ReplicatedVolumeHandle: "some-replication-identifier",
					},
				}
				replicationVolume = db.Volume{
					WorkerName: insertedWorker.Name,
					TTL:        5 * time.Minute,
					Handle:     "my-replication-handle",
					Identifier: replicationIdentifier,
				}

				err := database.InsertVolume(replicationVolume)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				savedReplicationVolumes, err := database.GetVolumesByIdentifier(replicationIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(savedReplicationVolumes)).To(Equal(1))

				savedReplicationVolume := savedReplicationVolumes[0]
				Expect(savedReplicationVolume.WorkerName).To(Equal(replicationVolume.WorkerName))
				Expect(savedReplicationVolume.TTL).To(Equal(replicationVolume.TTL))
				Expect(savedReplicationVolume.Handle).To(Equal(replicationVolume.Handle))
				Expect(savedReplicationVolume.Volume.Identifier.Replication.ReplicatedVolumeHandle).To(Equal("some-replication-identifier"))
				Expect(savedReplicationVolume.ExpiresIn).To(BeNumerically("~", replicationVolume.TTL, time.Second))
			})
		})

		Describe("import volumes", func() {
			var importVolume db.Volume
			var importIdentifier db.VolumeIdentifier
			var importIdentifierVersion string

			BeforeEach(func() {
				importIdentifierVersion = "some-version"
				importIdentifier = db.VolumeIdentifier{
					Import: &db.ImportIdentifier{
						WorkerName: insertedWorker.Name,
						Path:       "/some/path",
						Version:    &importIdentifierVersion,
					},
				}
				importVolume = db.Volume{
					WorkerName: insertedWorker.Name,
					TTL:        5 * time.Minute,
					Handle:     "my-import-handle",
					Identifier: importIdentifier,
				}

				err := database.InsertVolume(importVolume)
				Expect(err).NotTo(HaveOccurred())
			})

			It("can be retrieved", func() {
				savedImportVolumes, err := database.GetVolumesByIdentifier(importIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(savedImportVolumes)).To(Equal(1))
				savedImportVolume := savedImportVolumes[0]
				Expect(savedImportVolume.WorkerName).To(Equal(importVolume.WorkerName))
				Expect(savedImportVolume.TTL).To(Equal(importVolume.TTL))
				Expect(savedImportVolume.Handle).To(Equal(importVolume.Handle))
				Expect(savedImportVolume.Volume.Identifier.Import.WorkerName).To(Equal(insertedWorker.Name))
				Expect(savedImportVolume.Volume.Identifier.Import.Path).To(Equal("/some/path"))
				Expect(savedImportVolume.Volume.Identifier.Import.Version).To(Equal(&importIdentifierVersion))
				Expect(savedImportVolume.ExpiresIn).To(BeNumerically("~", importVolume.TTL, time.Second))
			})

			It("doesn't try to filter by version when the Version is nil", func() {
				identifierMissingVersion := db.VolumeIdentifier{
					Import: &db.ImportIdentifier{
						WorkerName: insertedWorker.Name,
						Path:       "/some/path",
					},
				}
				err := database.InsertVolume(db.Volume{
					WorkerName: insertedWorker.Name,
					TTL:        5 * time.Minute,
					Handle:     "my-other-import-handle",
					Identifier: identifierMissingVersion,
				})
				Expect(err).NotTo(HaveOccurred())

				volumes, err := database.GetVolumesByIdentifier(identifierMissingVersion)
				Expect(err).NotTo(HaveOccurred())
				handles := []string{}
				for i := range volumes {
					handles = append(handles, volumes[i].Handle)
				}
				Expect(handles).To(ConsistOf([]string{"my-import-handle", "my-other-import-handle"}))
			})
		})
	})

	Describe("GetVolumesForOneOffBuildImageResources", func() {
		It("returns all volumes containing image resource versions which were used in one-off builds", func() {
			oneOffBuildADB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			oneOffBuildBDB, err := teamDB.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			jobBuild, err := pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			By("returning volumes that are used in both one-off builds and job builds")
			volume1 := db.Volume{
				WorkerName: "some-worker",
				TTL:        2 * time.Minute,
				Handle:     "volume-1",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-1"},
						ResourceHash:    `docker:{"repository":"repository-1"}`,
					},
				},
			}
			err = database.InsertVolume(volume1)
			Expect(err).NotTo(HaveOccurred())
			err = oneOffBuildADB.SaveImageResourceVersion("plan-id-1", *volume1.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())
			err = jobBuild.SaveImageResourceVersion("plan-id-1", *volume1.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())

			By("returning more than one volume per build ID")
			volume2 := db.Volume{
				WorkerName: "some-worker",
				TTL:        2 * time.Minute,
				Handle:     "volume-2",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-2"},
						ResourceHash:    `docker:{"repository":"repository-2"}`,
					},
				},
			}
			err = database.InsertVolume(volume2)
			Expect(err).NotTo(HaveOccurred())
			err = oneOffBuildADB.SaveImageResourceVersion("plan-id-2", *volume2.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())

			By("returning more than one volume per VolumeIdentifier")
			volume3 := db.Volume{
				WorkerName: "some-worker",
				TTL:        2 * time.Minute,
				Handle:     "volume-3",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-1"},
						ResourceHash:    `docker:{"repository":"repository-1"}`,
					},
				},
			}
			err = database.InsertVolume(volume3)
			Expect(err).NotTo(HaveOccurred())
			err = oneOffBuildADB.SaveImageResourceVersion("plan-id-3", *volume3.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())

			By("returning volumes from multiple one-off builds")
			volume4 := db.Volume{
				WorkerName: "some-worker",
				TTL:        2 * time.Minute,
				Handle:     "volume-4",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-4"},
						ResourceHash:    `docker:{"repository":"repository-4"}`,
					},
				},
			}
			err = database.InsertVolume(volume4)
			Expect(err).NotTo(HaveOccurred())
			err = oneOffBuildBDB.SaveImageResourceVersion("plan-id-4", *volume4.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())

			By("ignoring volumes from job builds even if part of the VolumeIdentifier matches")
			volume5 := db.Volume{
				WorkerName: "some-worker",
				TTL:        2 * time.Minute,
				Handle:     "volume-5",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-1"},
						ResourceHash:    `docker:{"repository":"repository-2"}`,
					},
				},
			}
			err = database.InsertVolume(volume5)
			Expect(err).NotTo(HaveOccurred())
			err = jobBuild.SaveImageResourceVersion("plan-id-5", *volume5.Identifier.ResourceCache)
			Expect(err).NotTo(HaveOccurred())

			By("ignoring expired volumes")
			volume6 := db.Volume{
				WorkerName: "some-worker",
				TTL:        -time.Hour,
				Handle:     "volume-6",
				Identifier: db.VolumeIdentifier{
					ResourceCache: &db.ResourceCacheIdentifier{
						ResourceVersion: atc.Version{"digest": "digest-6"},
						ResourceHash:    `docker:{"repository":"repository-6"}`,
					},
				},
			}
			err = database.InsertVolume(volume6)
			Expect(err).NotTo(HaveOccurred())
			err = oneOffBuildADB.SaveImageResourceVersion("plan-id-6", *volume6.Identifier.ResourceCache)
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
