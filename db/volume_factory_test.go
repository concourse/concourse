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
		usedResourceCache, err = db.ForBuild(build.ID()).UseResourceCache(logger, setupTx, resourceCache)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())
	})

	Describe("GetTeamVolumes", func() {
		var (
			team1handles []string
			team2handles []string
		)

		JustBeforeEach(func() {
			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			team1handles = []string{}
			team2handles = []string{}

			team2, err = teamFactory.CreateTeam(atc.Team{Name: "some-other-defaultTeam"})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team2.ID(), defaultWorker.Name(), creatingContainer, "some-path-3")
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
			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			expectedCreatedHandles = []string{}
			expectedDestroyingHandles = []string{}

			creatingVolume1, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume3, err := createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())
			expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVolume3.Handle())

			resourceCacheVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-4")
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, resourceCacheVolume.Handle())

			resourceCacheVolumeCreated, err := resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			err = resourceCacheVolumeCreated.InitializeResourceCache(usedResourceCache)
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

			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
				Expect(vol.WorkerName()).To(Equal("default-worker"))
			}
			Expect(createdHandles).To(Equal(expectedCreatedHandles))

			destroyingHandles := []string{}
			for _, vol := range destoryingVolumes {
				destroyingHandles = append(destroyingHandles, vol.Handle())
				Expect(vol.WorkerName()).To(Equal("default-worker"))
			}

			Expect(destroyingHandles).To(Equal(expectedDestroyingHandles))
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

			usedResourceCache, err = db.ForResource(resource.ID()).UseResourceCache(logger, setupTx, cache)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume db.CreatedVolume

			BeforeEach(func() {
				var err error
				creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
					Type:     "get",
					StepName: "some-resource",
				})
				Expect(err).ToNot(HaveOccurred())

				resourceCacheVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path-4")
				Expect(err).NotTo(HaveOccurred())

				existingVolume, err = resourceCacheVolume.Created()
				Expect(err).NotTo(HaveOccurred())

				err = existingVolume.InitializeResourceCache(usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				createdVolume, found, err := volumeFactory.FindResourceCacheVolume(defaultWorker.Name(), usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(found).To(BeTrue())
			})
		})
	})
})
