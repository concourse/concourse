package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFactory", func() {
	var (
		team2             dbng.Team
		usedResourceCache *dbng.UsedResourceCache
		build             dbng.Build
	)

	BeforeEach(func() {
		build, err = defaultTeam.CreateOneOffBuild()
		Expect(err).ToNot(HaveOccurred())

		setupTx, err := dbConn.Begin()
		Expect(err).ToNot(HaveOccurred())
		defer setupTx.Rollback()

		baseResourceType := dbng.BaseResourceType{
			Name: "some-resource-type",
		}
		_, err = baseResourceType.FindOrCreate(setupTx)
		Expect(err).NotTo(HaveOccurred())

		resourceCache := dbng.ResourceCache{
			ResourceConfig: dbng.ResourceConfig{
				CreatedByBaseResourceType: &baseResourceType,
			},
		}
		usedResourceCache, err = resourceCache.FindOrCreateForBuild(logger, setupTx, lockFactory, build.ID())
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("GetTeamVolumes", func() {
		var (
			team1handles []string
			team2handles []string
		)

		JustBeforeEach(func() {
			creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			team1handles = []string{}
			team2handles = []string{}

			team2, err = teamFactory.CreateTeam("some-other-defaultTeam")
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
	})

	Describe("GetOrphanedVolumes", func() {
		var (
			expectedCreatedHandles    []string
			expectedDestroyingHandles []string
		)

		BeforeEach(func() {
			creatingContainer, err := defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
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
	})

	Describe("FindBaseResourceTypeVolume", func() {
		var usedBaseResourceType *dbng.UsedBaseResourceType
		BeforeEach(func() {
			baseResourceTypeFactory := dbng.NewBaseResourceTypeFactory(dbConn)
			var err error
			var found bool
			usedBaseResourceType, found, err = baseResourceTypeFactory.Find("some-resource-type")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
		})

		Context("when there is a created volume for base resource type", func() {
			var existingVolume dbng.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeFactory.CreateBaseResourceTypeVolume(defaultTeam.ID(), defaultWorker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(defaultTeam.ID(), defaultWorker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).To(BeNil())
				Expect(createdVolume).ToNot(BeNil())
				Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
			})
		})

		Context("when there is a creating volume for base resource type", func() {
			var existingVolume dbng.CreatingVolume

			BeforeEach(func() {
				var err error
				existingVolume, err = volumeFactory.CreateBaseResourceTypeVolume(defaultTeam.ID(), defaultWorker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(defaultTeam.ID(), defaultWorker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).ToNot(BeNil())
				Expect(creatingVolume.Handle()).To(Equal(existingVolume.Handle()))
				Expect(createdVolume).To(BeNil())
			})
		})
	})

	Describe("FindResourceCacheVolume", func() {
		var usedResourceCache *dbng.UsedResourceCache

		BeforeEach(func() {
			resource, err := defaultPipeline.CreateResource("some-resource", atc.ResourceConfig{})
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			baseResourceType := dbng.BaseResourceType{
				Name: "some-base-type",
			}
			_, err = baseResourceType.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())

			cache := dbng.ResourceCache{
				ResourceConfig: dbng.ResourceConfig{
					CreatedByBaseResourceType: &baseResourceType,

					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
				Params:  atc.Params{"some": "params"},
			}

			usedResourceCache, err = cache.FindOrCreateForResource(logger, setupTx, lockFactory, resource.ID)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume dbng.CreatedVolume

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
			var existingVolume dbng.CreatingVolume

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
		var usedResourceCache *dbng.UsedResourceCache

		BeforeEach(func() {
			resource, err := defaultPipeline.CreateResource("some-resource", atc.ResourceConfig{})
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())

			baseResourceType := dbng.BaseResourceType{
				Name: "some-base-type",
			}
			_, err = baseResourceType.FindOrCreate(setupTx)
			Expect(err).NotTo(HaveOccurred())

			cache := dbng.ResourceCache{
				ResourceConfig: dbng.ResourceConfig{
					CreatedByBaseResourceType: &baseResourceType,

					Source: atc.Source{"some": "source"},
				},
				Version: atc.Version{"some": "version"},
				Params:  atc.Params{"some": "params"},
			}

			usedResourceCache, err = cache.FindOrCreateForResource(logger, setupTx, lockFactory, resource.ID)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())
		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume dbng.CreatedVolume

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
})
