package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeRepository", func() {
	var (
		team2             db.Team
		usedResourceCache db.UsedResourceCache
		build             db.Build
	)

	BeforeEach(func() {
		var err error
		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		usedResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
			db.ForBuild(build.ID()),
			"some-type",
			atc.Version{"some": "version"},
			atc.Source{
				"some": "source",
			},
			atc.Params{"some": "params"},
			atc.VersionedResourceTypes{
				atc.VersionedResourceType{
					ResourceType: atc.ResourceType{
						Name: "some-type",
						Type: "some-base-resource-type",
						Source: atc.Source{
							"some-type": "source",
						},
					},
					Version: atc.Version{"some-type": "version"},
				},
			},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetTeamVolumes", func() {
		var (
			team1handles []string
			team2handles []string
		)

		It("returns task cache volumes", func() {
			taskCache, err := taskCacheFactory.FindOrCreate(defaultJob.ID(), "some-step", "some-path")
			Expect(err).NotTo(HaveOccurred())

			usedWorkerTaskCache, err := workerTaskCacheFactory.FindOrCreate(db.WorkerTaskCache{
				TaskCache:  taskCache,
				WorkerName: defaultWorker.Name(),
			})
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, err := volumeRepository.CreateTaskCacheVolume(defaultTeam.ID(), usedWorkerTaskCache)
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			volumes, err := volumeRepository.GetTeamVolumes(defaultTeam.ID())
			Expect(err).NotTo(HaveOccurred())

			Expect(volumes).To(HaveLen(1))
			Expect(volumes[0].Handle()).To(Equal(createdVolume.Handle()))
			Expect(volumes[0].Type()).To(Equal(db.VolumeTypeTaskCache))
		})

		Context("with container volumes", func() {
			JustBeforeEach(func() {
				creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()), db.ContainerMetadata{
					Type:     "task",
					StepName: "some-task",
				})
				Expect(err).ToNot(HaveOccurred())

				team1handles = []string{}
				team2handles = []string{}

				team2, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-defaultTeam"})
				Expect(err).ToNot(HaveOccurred())

				creatingVolume1, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
				Expect(err).NotTo(HaveOccurred())
				createdVolume1, err := creatingVolume1.Created()
				Expect(err).NotTo(HaveOccurred())
				team1handles = append(team1handles, createdVolume1.Handle())

				creatingVolume2, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
				Expect(err).NotTo(HaveOccurred())
				createdVolume2, err := creatingVolume2.Created()
				Expect(err).NotTo(HaveOccurred())
				team1handles = append(team1handles, createdVolume2.Handle())

				creatingVolume3, err := volumeRepository.CreateContainerVolume(team2.ID(), defaultWorker.Name(), creatingContainer, "some-path-3")
				Expect(err).NotTo(HaveOccurred())
				createdVolume3, err := creatingVolume3.Created()
				Expect(err).NotTo(HaveOccurred())
				team2handles = append(team2handles, createdVolume3.Handle())
			})

			It("returns only the matching defaultTeam's volumes", func() {
				createdVolumes, err := volumeRepository.GetTeamVolumes(defaultTeam.ID())
				Expect(err).NotTo(HaveOccurred())
				createdHandles := []string{}
				for _, vol := range createdVolumes {
					createdHandles = append(createdHandles, vol.Handle())
				}
				Expect(createdHandles).To(Equal(team1handles))

				createdVolumes2, err := volumeRepository.GetTeamVolumes(team2.ID())
				Expect(err).NotTo(HaveOccurred())
				createdHandles2 := []string{}
				for _, vol := range createdVolumes2 {
					createdHandles2 = append(createdHandles2, vol.Handle())
				}
				Expect(createdHandles2).To(Equal(team2handles))
			})

			Context("when worker is stalled", func() {
				BeforeEach(func() {
					var err error
					defaultWorker, err = workerFactory.SaveWorker(defaultWorkerPayload, -10*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
					Expect(stalledWorkers).To(ContainElement(defaultWorker.Name()))
				})

				It("returns volumes", func() {
					createdVolumes, err := volumeRepository.GetTeamVolumes(defaultTeam.ID())
					Expect(err).NotTo(HaveOccurred())
					createdHandles := []string{}
					for _, vol := range createdVolumes {
						createdHandles = append(createdHandles, vol.Handle())
					}
					Expect(createdHandles).To(Equal(team1handles))

					createdVolumes2, err := volumeRepository.GetTeamVolumes(team2.ID())
					Expect(err).NotTo(HaveOccurred())
					createdHandles2 := []string{}
					for _, vol := range createdVolumes2 {
						createdHandles2 = append(createdHandles2, vol.Handle())
					}
					Expect(createdHandles2).To(Equal(team2handles))
				})
			})
		})
	})

	Describe("GetOrphanedVolumes", func() {
		var (
			expectedCreatedHandles    []string
			expectedDestroyingHandles []string
			certsVolumeHandle         string
		)

		BeforeEach(func() {
			creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())
			expectedCreatedHandles = []string{}
			expectedDestroyingHandles = []string{}

			creatingVolume1, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume1.Handle())

			creatingVolume2, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume2.Handle())

			creatingVolume3, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume3, err := createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())
			expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVolume3.Handle())

			creatingVolumeOtherWorker, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), otherWorker.Name(), creatingContainer, "some-path-other-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolumeOtherWorker, err := creatingVolumeOtherWorker.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolumeOtherWorker.Handle())

			resourceCacheVolume, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-4")
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, resourceCacheVolume.Handle())

			resourceCacheVolumeCreated, err := resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			err = resourceCacheVolumeCreated.InitializeResourceCache(usedResourceCache)
			Expect(err).NotTo(HaveOccurred())

			artifactVolume, err := volumeRepository.CreateVolume(defaultTeam.ID(), defaultWorker.Name(), db.VolumeTypeArtifact)
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, artifactVolume.Handle())

			_, err = artifactVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			usedWorkerBaseResourceType, found, err := workerBaseResourceTypeFactory.Find(defaultWorkerResourceType.Type, defaultWorker)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			baseResourceTypeVolume, err := volumeRepository.CreateBaseResourceTypeVolume(usedWorkerBaseResourceType)
			Expect(err).NotTo(HaveOccurred())

			oldResourceTypeVolume, err := baseResourceTypeVolume.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, oldResourceTypeVolume.Handle())

			newVersion := defaultWorkerResourceType
			newVersion.Version = "some-new-brt-version"

			newWorker := defaultWorkerPayload
			newWorker.ResourceTypes = []atc.WorkerResourceType{newVersion}

			defaultWorker, err = workerFactory.SaveWorker(newWorker, 0)
			Expect(err).ToNot(HaveOccurred())

			tx, err := dbConn.Begin()
			Expect(err).NotTo(HaveOccurred())
			workerResourceCerts, err := db.WorkerResourceCerts{
				WorkerName: defaultWorker.Name(),
				CertsPath:  "/etc/blah/blah/certs",
			}.FindOrCreate(tx)
			Expect(err).NotTo(HaveOccurred())
			err = tx.Commit()
			Expect(err).NotTo(HaveOccurred())

			certsVolume, err := volumeRepository.CreateResourceCertsVolume(defaultWorker.Name(), workerResourceCerts)
			Expect(err).NotTo(HaveOccurred())

			certsVolumeHandle = certsVolume.Handle()

			deleted, err := build.Delete()
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			deleteTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			deleted, err = usedResourceCache.Destroy(deleteTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
			Expect(deleteTx.Commit()).To(Succeed())

			createdContainer, err := creatingContainer.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingContainer, err := createdContainer.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingContainer.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(BeTrue())
		})

		It("returns orphaned volumes", func() {
			createdVolumes, err := volumeRepository.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			createdHandles := []string{}

			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
			}
			Expect(createdHandles).To(ConsistOf(expectedCreatedHandles))
			Expect(createdHandles).ToNot(ContainElement(certsVolumeHandle))
		})

		Context("when worker is stalled", func() {
			BeforeEach(func() {
				var err error
				defaultWorker, err = workerFactory.SaveWorker(defaultWorkerPayload, -11*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				stalledWorkers, err := workerLifecycle.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(stalledWorkers).To(ContainElement(defaultWorker.Name()))
			})

			It("does not return volumes from stalled worker", func() {
				createdVolumes, err := volumeRepository.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())

				for _, v := range createdVolumes {
					Expect(v.WorkerName()).ToNot(Equal(defaultWorker.Name()))
				}
			})
		})

		Context("when worker is landed", func() {
			BeforeEach(func() {
				err := defaultWorker.Land()
				Expect(err).NotTo(HaveOccurred())
				landedWorkers, err := workerLifecycle.LandFinishedLandingWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(landedWorkers).To(ContainElement(defaultWorker.Name()))
			})

			It("does not return volumes for the worker", func() {
				createdVolumes, err := volumeRepository.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())

				for _, v := range createdVolumes {
					Expect(v.WorkerName()).ToNot(Equal(defaultWorker.Name()))
				}
			})
		})
	})

	Describe("DestroyFailedVolumes", func() {
		BeforeEach(func() {
			creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			_, err = creatingVolume1.Failed()
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns length of failed volumes", func() {
			failedVolumes, err := volumeRepository.DestroyFailedVolumes()
			Expect(err).NotTo(HaveOccurred())
			Expect(failedVolumes).To(Equal(1))
		})
	})

	Describe("GetDestroyingVolumes", func() {
		var expectedDestroyingHandles []string
		var destroyingVol db.DestroyingVolume

		Context("when worker has detroying volumes", func() {
			BeforeEach(func() {
				creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()), db.ContainerMetadata{
					Type:     "task",
					StepName: "some-task",
				})
				Expect(err).ToNot(HaveOccurred())

				expectedDestroyingHandles = []string{}

				creatingVol, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
				Expect(err).NotTo(HaveOccurred())

				createdVol, err := creatingVol.Created()
				Expect(err).NotTo(HaveOccurred())

				destroyingVol, err = createdVol.Destroying()
				Expect(err).NotTo(HaveOccurred())

				expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVol.Handle())
			})

			It("returns destroying volumes", func() {
				destroyingVolumes, err := volumeRepository.GetDestroyingVolumes(defaultWorker.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(destroyingVolumes).To(Equal(expectedDestroyingHandles))
			})
			Context("when worker doesn't have detroying volume", func() {
				BeforeEach(func() {
					deleted, err := destroyingVol.Destroy()
					Expect(err).NotTo(HaveOccurred())
					Expect(deleted).To(BeTrue())
				})

				It("returns empty volumes", func() {
					destroyingVolumes, err := volumeRepository.GetDestroyingVolumes(defaultWorker.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(destroyingVolumes).To(BeEmpty())
				})
			})
		})
	})

	Describe("CreateBaseResourceTypeVolume", func() {
		var usedWorkerBaseResourceType *db.UsedWorkerBaseResourceType
		BeforeEach(func() {
			workerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
			var err error
			var found bool
			usedWorkerBaseResourceType, found, err = workerBaseResourceTypeFactory.Find("some-base-resource-type", defaultWorker)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		It("creates a CreatingVolume with no team ID set", func() {
			volume, err := volumeRepository.CreateBaseResourceTypeVolume(usedWorkerBaseResourceType)
			Expect(err).NotTo(HaveOccurred())
			var teamID int
			err = psql.Select("team_id").From("volumes").
				Where(sq.Eq{"handle": volume.Handle()}).RunWith(dbConn).QueryRow().Scan(&teamID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Scan error"))
		})
	})

	Describe("CreateVolume", func() {
		It("creates a CreatingVolume of the given type with a teamID", func() {
			volume, err := volumeRepository.CreateVolume(defaultTeam.ID(), defaultWorker.Name(), db.VolumeTypeArtifact)
			Expect(err).NotTo(HaveOccurred())
			var teamID int
			var workerName string
			err = psql.Select("team_id, worker_name").From("volumes").
				Where(sq.Eq{"handle": volume.Handle()}).RunWith(dbConn).QueryRow().Scan(&teamID, &workerName)
			Expect(err).NotTo(HaveOccurred())
			Expect(teamID).To(Equal(defaultTeam.ID()))
			Expect(workerName).To(Equal(defaultWorker.Name()))
		})
	})

	Describe("FindBaseResourceTypeVolume", func() {
		var usedWorkerBaseResourceType *db.UsedWorkerBaseResourceType
		BeforeEach(func() {
			workerBaseResourceTypeFactory := db.NewWorkerBaseResourceTypeFactory(dbConn)
			var err error
			var found bool
			usedWorkerBaseResourceType, found, err = workerBaseResourceTypeFactory.Find("some-base-resource-type", defaultWorker)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when there is a created volume for base resource type", func() {
			var existingVolume db.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeRepository.CreateBaseResourceTypeVolume(usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeRepository.FindBaseResourceTypeVolume(usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).To(BeNil())
				Expect(createdVolume).ToNot(BeNil())
				Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
			})
		})

		Context("when there is a creating volume for base resource type", func() {
			var existingVolume db.CreatingVolume

			BeforeEach(func() {
				var err error
				existingVolume, err = volumeRepository.CreateBaseResourceTypeVolume(usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeRepository.FindBaseResourceTypeVolume(usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).ToNot(BeNil())
				Expect(creatingVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(createdVolume).To(BeNil())
			})
		})
	})

	Describe("FindResourceCacheVolume", func() {
		var usedResourceCache db.UsedResourceCache

		BeforeEach(func() {
			build, err := defaultPipeline.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			usedResourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					atc.VersionedResourceType{
						ResourceType: atc.ResourceType{
							Name: "some-type",
							Type: "some-base-resource-type",
							Source: atc.Source{
								"some-type": "source",
							},
						},
						Version: atc.Version{"some-type": "version"},
					},
				},
			)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume db.CreatedVolume

			BeforeEach(func() {
				var err error
				creatingContainer, err := defaultWorker.CreateContainer(db.NewBuildStepContainerOwner(build.ID(), "some-plan", defaultTeam.ID()), db.ContainerMetadata{
					Type:     "get",
					StepName: "some-resource",
				})
				Expect(err).ToNot(HaveOccurred())

				resourceCacheVolume, err := volumeRepository.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-4")
				Expect(err).NotTo(HaveOccurred())

				existingVolume, err = resourceCacheVolume.Created()
				Expect(err).NotTo(HaveOccurred())

				err = existingVolume.InitializeResourceCache(usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				createdVolume, found, err := volumeRepository.FindResourceCacheVolume(defaultWorker.Name(), usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("RemoveDestroyingVolumes", func() {
		var failedErr error
		var numDeleted int
		var handles []string

		JustBeforeEach(func() {
			numDeleted, failedErr = volumeRepository.RemoveDestroyingVolumes(defaultWorker.Name(), handles)
		})

		Context("when there are volumes to destroy", func() {

			Context("when volume is in destroying state", func() {
				BeforeEach(func() {
					handles = []string{"some-handle1", "some-handle2"}
					result, err := psql.Insert("volumes").SetMap(map[string]interface{}{
						"state":       "destroying",
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("should destroy", func() {
					result, err := psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(0)))
				})
				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(1))
				})
				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when handles are empty list", func() {
				BeforeEach(func() {
					handles = []string{}
					result, err := psql.Insert("volumes").SetMap(map[string]interface{}{
						"state":       "destroying",
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("should destroy", func() {
					result, err := psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(0)))
				})
				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(1))
				})
				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when volume is in create/creating state", func() {
				BeforeEach(func() {
					handles = []string{"some-handle1", "some-handle2"}
					result, err := psql.Insert("volumes").SetMap(map[string]interface{}{
						"state":       "creating",
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("should not destroy", func() {
					result, err := psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(0))
				})
				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when there are no volumes to destroy", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}

				result, err := psql.Insert("volumes").SetMap(
					map[string]interface{}{
						"state":       "destroying",
						"handle":      "some-handle1",
						"worker_name": defaultWorker.Name(),
					},
				).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Insert("volumes").SetMap(
					map[string]interface{}{
						"state":       "destroying",
						"handle":      "some-handle2",
						"worker_name": defaultWorker.Name(),
					},
				).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("doesn't destroy volumes that are in handles", func() {
				result, err := psql.Select("*").From("volumes").
					Where(sq.Eq{"handle": handles}).RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(2)))
			})

			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})
			It("returns the correct number of rows removed", func() {
				Expect(numDeleted).To(Equal(0))
			})
		})
	})

	Describe("RemoveMissingVolumes", func() {
		var (
			today        time.Time
			gracePeriod  time.Duration
			rowsAffected int
			err          error
		)

		JustBeforeEach(func() {
			rowsAffected, err = volumeRepository.RemoveMissingVolumes(gracePeriod)
		})

		Context("when there are multiple volumes with varying missing since times", func() {
			BeforeEach(func() {
				today = time.Now()

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":      "some-handle-1",
					"state":       db.VolumeStateCreated,
					"worker_name": defaultWorker.Name(),
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":        "some-handle-2",
					"state":         db.VolumeStateCreated,
					"worker_name":   otherWorker.Name(),
					"missing_since": today,
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":        "some-handle-3",
					"state":         db.VolumeStateFailed,
					"worker_name":   otherWorker.Name(),
					"missing_since": today.Add(-5 * time.Minute),
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":        "some-handle-4",
					"state":         db.VolumeStateDestroying,
					"worker_name":   defaultWorker.Name(),
					"missing_since": today.Add(-10 * time.Minute),
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when no created/failed volumes have expired", func() {
				BeforeEach(func() {
					gracePeriod = 7 * time.Minute
				})

				It("affects no volumes", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(rowsAffected).To(Equal(0))
				})
			})

			Context("when some created/failed volumes have expired", func() {
				BeforeEach(func() {
					gracePeriod = 3 * time.Minute
				})

				It("affects some volumes", func() {
					Expect(err).ToNot(HaveOccurred())
					Expect(rowsAffected).To(Equal(1))
				})

				It("affects the right volumes", func() {
					result, err := psql.Select("*").From("volumes").
						RunWith(dbConn).Exec()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(3)))

					result, err = psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "some-handle-1"}).RunWith(dbConn).Exec()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))

					result, err = psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "some-handle-2"}).RunWith(dbConn).Exec()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))

					result, err = psql.Select("*").From("volumes").
						Where(sq.Eq{"handle": "some-handle-4"}).RunWith(dbConn).Exec()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
			})
		})

		Context("when there is a missing parent volume", func() {
			BeforeEach(func() {
				today = time.Now()

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":      "alive-handle",
					"state":       db.VolumeStateCreated,
					"worker_name": defaultWorker.Name(),
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				var parentID int
				err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":        "parent-handle",
					"state":         db.VolumeStateCreated,
					"worker_name":   defaultWorker.Name(),
					"missing_since": today.Add(-10 * time.Minute),
				}).Suffix("RETURNING id").RunWith(dbConn).QueryRow().Scan(&parentID)
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Insert("volumes").SetMap(map[string]interface{}{
					"handle":      "child-handle",
					"state":       db.VolumeStateCreated,
					"worker_name": defaultWorker.Name(),
					"parent_id":   parentID,
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				gracePeriod = 3 * time.Minute
			})

			It("affects some volumes", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(rowsAffected).To(Equal(2))
			})

			It("removes the child and missing parent volume", func() {
				var volumeCount int
				err = psql.Select("COUNT(id)").From("volumes").RunWith(dbConn).QueryRow().Scan(&volumeCount)
				Expect(err).ToNot(HaveOccurred())
				Expect(volumeCount).To(Equal(1))

				result, err := psql.Select("*").From("volumes").
					Where(sq.Eq{"handle": "parent-handle"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(0)))

				result, err = psql.Select("*").From("volumes").
					Where(sq.Eq{"handle": "child-handle"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(0)))
			})
		})
	})

	Describe("UpdateVolumesMissingSince", func() {
		var (
			today        time.Time
			err          error
			handles      []string
			missingSince pq.NullTime
		)

		BeforeEach(func() {
			result, err := psql.Insert("volumes").SetMap(map[string]interface{}{
				"state":       db.VolumeStateDestroying,
				"handle":      "some-handle1",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			result, err = psql.Insert("volumes").SetMap(map[string]interface{}{
				"state":       db.VolumeStateDestroying,
				"handle":      "some-handle2",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			today = time.Date(2018, 9, 24, 0, 0, 0, 0, time.UTC)

			result, err = psql.Insert("volumes").SetMap(map[string]interface{}{
				"state":         db.VolumeStateCreated,
				"handle":        "some-handle3",
				"worker_name":   defaultWorker.Name(),
				"missing_since": today,
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))
		})

		JustBeforeEach(func() {
			err = volumeRepository.UpdateVolumesMissingSince(defaultWorker.Name(), handles)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when the reported handles is a subset", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1"}
			})

			Context("having the volumes in the creating state in the db", func() {
				BeforeEach(func() {
					result, err := psql.Update("volumes").
						Where(sq.Eq{"handle": "some-handle3"}).
						SetMap(map[string]interface{}{
							"state":         db.VolumeStateCreating,
							"missing_since": nil,
						}).RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})

				It("does not mark as missing", func() {
					err = psql.Select("missing_since").From("volumes").
						Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
					Expect(err).ToNot(HaveOccurred())
					Expect(missingSince.Valid).To(BeFalse())
				})
			})

			It("should mark volumes not in the subset and not already marked as missing", func() {
				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeTrue())

				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeTrue())
				Expect(missingSince.Time.Unix()).To(Equal(today.Unix()))
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the reported handles is the full set", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}
			})

			It("should not update", func() {
				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the reported handles includes a volume marked as missing", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2", "some-handle3"}
			})

			It("should mark the previously missing volume as not missing", func() {
				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("volumes").
					Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("DestroyUnknownVolumes", func() {
		var (
			err                   error
			workerReportedHandles []string
			num                   int
		)

		BeforeEach(func() {
			result, err := psql.Insert("volumes").SetMap(map[string]interface{}{
				"state":       db.VolumeStateDestroying,
				"handle":      "some-handle1",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			result, err = psql.Insert("volumes").SetMap(map[string]interface{}{
				"state":       db.VolumeStateCreated,
				"handle":      "some-handle2",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))
		})

		JustBeforeEach(func() {
			num, err = volumeRepository.DestroyUnknownVolumes(defaultWorker.Name(), workerReportedHandles)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there are volumes on the worker that are not in the db", func() {
			var destroyingVolumeHandles []string
			BeforeEach(func() {
				workerReportedHandles = []string{"some-handle3", "some-handle4"}
				destroyingVolumeHandles = append(workerReportedHandles, "some-handle1")
			})

			It("adds new destroying volumes to the database", func() {
				result, err := psql.Select("handle").
					From("volumes").
					Where(sq.Eq{"state": db.VolumeStateDestroying}).
					RunWith(dbConn).Query()

				Expect(err).ToNot(HaveOccurred())

				var handle string
				for result.Next() {
					err = result.Scan(&handle)
					Expect(err).ToNot(HaveOccurred())
					Expect(handle).Should(BeElementOf(destroyingVolumeHandles))
				}
				Expect(num).To(Equal(2))
			})

			It("does not affect volumes in any other state", func() {
				result, err := psql.Select("*").
					From("volumes").
					Where(sq.Eq{"state": db.VolumeStateCreated}).
					RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
				Expect(num).To(Equal(2))
			})
		})

		Context("when there are no unknown volumes on the worker", func() {
			BeforeEach(func() {
				workerReportedHandles = []string{"some-handle1", "some-handle2"}
			})

			It("should not try to destroy anything", func() {
				Expect(num).To(Equal(0))
				result, err := psql.Select("handle, state").
					From("volumes").
					Where(sq.Eq{"state": db.VolumeStateDestroying}).
					RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})
		})
	})
})
