package db_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFactory", func() {
	var (
		team2             db.Team
		usedResourceCache *db.UsedResourceCache
		baseResourceType  db.BaseResourceType
		build             db.Build
	)

	BeforeEach(func() {
		var err error
		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())
		defer setupTx.Rollback()

		baseResourceType = db.BaseResourceType{
			Name: "some-base-resource-type",
		}

		resourceCache := db.ResourceCache{
			ResourceConfig: db.ResourceConfig{
				CreatedByBaseResourceType: &baseResourceType,
			},
		}
		usedResourceCache, err = db.ForBuild(build.ID()).UseResourceCache(logger, setupTx, lockFactory, resourceCache)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())
	})

	Describe("GetTeamVolumes", func() {
		var (
			team1handles []string
			team2handles []string
		)

		JustBeforeEach(func() {
			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.ForBuild(build.ID()), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			team1handles = []string{}
			team2handles = []string{}

			team2, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-defaultTeam"})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team2.ID(), defaultWorker, creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			team2handles = append(team2handles, createdVolume3.Handle())
		})

		It("returns only the matching defaultTeam's volumes", func() {
			createdVolumes, err := volumeFactory.GetTeamVolumes(defaultTeam.ID())
			Expect(err).NotTo(HaveOccurred())
			createdHandles := []string{}
			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
			}
			Expect(createdHandles).To(Equal(team1handles))

			createdVolumes2, err := volumeFactory.GetTeamVolumes(team2.ID())
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
				createdVolumes, err := volumeFactory.GetTeamVolumes(defaultTeam.ID())
				Expect(err).NotTo(HaveOccurred())
				createdHandles := []string{}
				for _, vol := range createdVolumes {
					createdHandles = append(createdHandles, vol.Handle())
				}
				Expect(createdHandles).To(Equal(team1handles))

				createdVolumes2, err := volumeFactory.GetTeamVolumes(team2.ID())
				Expect(err).NotTo(HaveOccurred())
				createdHandles2 := []string{}
				for _, vol := range createdVolumes2 {
					createdHandles2 = append(createdHandles2, vol.Handle())
				}
				Expect(createdHandles2).To(Equal(team2handles))
			})
		})
	})

	Describe("GetOrphanedVolumes", func() {
		var (
			expectedCreatedHandles    []string
			expectedDestroyingHandles []string
		)

		BeforeEach(func() {
			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.ForBuild(build.ID()), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			expectedCreatedHandles = []string{}
			expectedDestroyingHandles = []string{}

			creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume3, err := createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())
			expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVolume3.Handle())

			resourceCacheVolume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
			Expect(err).NotTo(HaveOccurred())

			_, err = resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

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
			createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
			Expect(err).NotTo(HaveOccurred())
			createdHandles := []string{}
			expectAddr := "5.6.7.8:7878"

			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
				Expect(*vol.Worker().BaggageclaimURL()).To(Equal(expectAddr))
			}
			Expect(createdHandles).To(Equal(expectedCreatedHandles))

			destroyingHandles := []string{}
			for _, vol := range destoryingVolumes {
				destroyingHandles = append(destroyingHandles, vol.Handle())
				Expect(*vol.Worker().BaggageclaimURL()).To(Equal(expectAddr))
			}
			Expect(destroyingHandles).To(Equal(destroyingHandles))
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

			It("does not return volumes", func() {
				createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(0))
				Expect(destoryingVolumes).To(HaveLen(0))
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

			It("does not return volumes", func() {
				createdVolumes, destoryingVolumes, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolumes).To(HaveLen(0))
				Expect(destoryingVolumes).To(HaveLen(0))
			})
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
				volume, err := volumeFactory.CreateBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
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
				existingVolume, err = volumeFactory.CreateBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).ToNot(BeNil())
				Expect(creatingVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(createdVolume).To(BeNil())
			})
		})
	})

	Describe("FindResourceCacheVolume", func() {
		var usedResourceCache *db.UsedResourceCache

		BeforeEach(func() {
			resource, found, err := defaultPipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			cache := db.ResourceCache{
				ResourceConfig: db.ResourceConfig{
					CreatedByBaseResourceType: &baseResourceType,

					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
				Params:  atc.Params{"some": "params"},
			}

			usedResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, setupTx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume db.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindResourceCacheVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).To(BeNil())
				Expect(createdVolume).ToNot(BeNil())
				Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
			})
		})

		Context("when there is a creating volume for resource cache", func() {
			var existingVolume db.CreatingVolume

			BeforeEach(func() {
				var err error
				existingVolume, err = volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindResourceCacheVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).ToNot(BeNil())
				Expect(creatingVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(createdVolume).To(BeNil())
			})
		})
	})

	Describe("FindResourceCacheInitializedVolume", func() {
		var usedResourceCache *db.UsedResourceCache

		BeforeEach(func() {
			resource, found, err := defaultPipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			cache := db.ResourceCache{
				ResourceConfig: db.ResourceConfig{
					CreatedByBaseResourceType: &baseResourceType,

					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
				Params:  atc.Params{"some": "params"},
			}

			usedResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, setupTx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())
		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume db.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when volume is initialized", func() {
				BeforeEach(func() {
					err := existingVolume.Initialize()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns created volume", func() {
					createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(defaultWorker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(createdVolume).ToNot(BeNil())
					Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
				})
			})

			Context("when volume is uninitialized", func() {
				It("does not return volume", func() {
					createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(defaultWorker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(createdVolume).To(BeNil())
				})
			})
		})

		Context("when there is no created volume for resource cache", func() {
			It("does not return volume", func() {
				createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(defaultWorker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(createdVolume).To(BeNil())
			})
		})
	})

	Describe("GetDuplicateResourceCacheVolumes", func() {
		var (
			usedResourceCache   *db.UsedResourceCache
			uninitializedVolume db.CreatingVolume
			resource            db.Resource
		)

		BeforeEach(func() {
			var err error
			var found bool
			resource, found, err = defaultPipeline.Resource("some-resource")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			cache := db.ResourceCache{
				ResourceConfig: db.ResourceConfig{
					CreatedByBaseResourceType: &baseResourceType,

					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
				Params:  atc.Params{"some": "params"},
			}

			usedResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, setupTx, lockFactory, cache)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

			uninitializedVolume, err = volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there is a duplicate volume for resource cache", func() {
			var duplicateVolume db.CreatingVolume

			Context("where volume is on the same worker", func() {
				BeforeEach(func() {
					var err error
					duplicateVolume, err = volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when volume is not initialized and other volume is initialized", func() {
					BeforeEach(func() {
						createdVolume, err := duplicateVolume.Created()
						Expect(err).NotTo(HaveOccurred())
						err = createdVolume.Initialize()
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when volume in creating state", func() {
						It("returns uninitialized volume", func() {
							creatingVolumes, _, _, err := volumeFactory.GetDuplicateResourceCacheVolumes()
							Expect(err).NotTo(HaveOccurred())
							Expect(creatingVolumes).To(HaveLen(1))
							Expect(creatingVolumes[0].Handle()).To(Equal(uninitializedVolume.Handle()))
						})
					})

					Context("when volume in created state", func() {
						BeforeEach(func() {
							_, err := uninitializedVolume.Created()
							Expect(err).NotTo(HaveOccurred())
						})

						It("returns uninitialized volume", func() {
							_, createdVolumes, _, err := volumeFactory.GetDuplicateResourceCacheVolumes()
							Expect(err).NotTo(HaveOccurred())
							Expect(createdVolumes).To(HaveLen(1))
							Expect(createdVolumes[0].Handle()).To(Equal(uninitializedVolume.Handle()))
						})
					})

					Context("when volume in destroying state", func() {
						BeforeEach(func() {
							createdVolume, err := uninitializedVolume.Created()
							Expect(err).NotTo(HaveOccurred())
							_, err = createdVolume.Destroying()
							Expect(err).NotTo(HaveOccurred())
						})

						It("returns uninitialized volume", func() {
							_, _, destroyingVolumes, err := volumeFactory.GetDuplicateResourceCacheVolumes()
							Expect(err).NotTo(HaveOccurred())
							Expect(destroyingVolumes).To(HaveLen(1))
							Expect(destroyingVolumes[0].Handle()).To(Equal(uninitializedVolume.Handle()))
						})
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

						It("does not return volumes", func() {
							creatingVolumes, _, _, err := volumeFactory.GetDuplicateResourceCacheVolumes()
							Expect(err).NotTo(HaveOccurred())
							Expect(creatingVolumes).To(HaveLen(0))
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

						It("does not return volumes", func() {
							creatingVolumes, _, _, err := volumeFactory.GetDuplicateResourceCacheVolumes()
							Expect(err).NotTo(HaveOccurred())
							Expect(creatingVolumes).To(HaveLen(0))
						})
					})
				})

				Context("when both volumes are not initialized", func() {
					It("does not return any volume", func() {
						creatingVolumes, createdVolumes, destroyingVolumes, err := volumeFactory.GetDuplicateResourceCacheVolumes()
						Expect(err).NotTo(HaveOccurred())
						Expect(creatingVolumes).To(HaveLen(0))
						Expect(createdVolumes).To(HaveLen(0))
						Expect(destroyingVolumes).To(HaveLen(0))
					})
				})
			})

			Context("when volume is on different worker", func() {
				BeforeEach(func() {
					anotherWorker, err := workerFactory.SaveWorker(atc.Worker{
						GardenAddr:      "some-garden-addr",
						BaggageclaimURL: "some-bc-url",
						ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
					}, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					_, err = volumeFactory.CreateResourceCacheVolume(anotherWorker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not return any volume", func() {
					creatingVolumes, createdVolumes, destroyingVolumes, err := volumeFactory.GetDuplicateResourceCacheVolumes()
					Expect(err).NotTo(HaveOccurred())
					Expect(creatingVolumes).To(HaveLen(0))
					Expect(createdVolumes).To(HaveLen(0))
					Expect(destroyingVolumes).To(HaveLen(0))
				})
			})
		})

		Context("when there is volume for different resource cache", func() {
			BeforeEach(func() {
				setupTx, err := dbConn.Begin()
				Expect(err).ToNot(HaveOccurred())
				anotherResourceCache := db.ResourceCache{
					ResourceConfig: db.ResourceConfig{
						CreatedByBaseResourceType: &baseResourceType,

						Source: atc.Source{"some": "source"},
					},
					Version: atc.Version{"some": "version"},
					Params:  atc.Params{"some": "params"},
				}

				usedResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, setupTx, lockFactory, anotherResourceCache)
				Expect(err).ToNot(HaveOccurred())

				Expect(setupTx.Commit()).To(Succeed())
			})

			It("does not return any volume", func() {
				creatingVolumes, createdVolumes, destroyingVolumes, err := volumeFactory.GetDuplicateResourceCacheVolumes()
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolumes).To(HaveLen(0))
				Expect(createdVolumes).To(HaveLen(0))
				Expect(destroyingVolumes).To(HaveLen(0))
			})
		})
	})
})
