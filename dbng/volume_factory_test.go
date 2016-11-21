package dbng_test

import (
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VolumeFactory", func() {
	var (
		dbConn            dbng.Conn
		volumeFactory     dbng.VolumeFactory
		containerFactory  *dbng.ContainerFactory
		teamFactory       dbng.TeamFactory
		buildFactory      *dbng.BuildFactory
		team              *dbng.Team
		team2             *dbng.Team
		worker            *dbng.Worker
		usedResourceCache *dbng.UsedResourceCache
		build             *dbng.Build
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		containerFactory = dbng.NewContainerFactory(dbConn)
		volumeFactory = dbng.NewVolumeFactory(dbConn)
		teamFactory = dbng.NewTeamFactory(dbConn)
		buildFactory = dbng.NewBuildFactory(dbConn)

		var err error
		team, err = teamFactory.CreateTeam("some-team")
		Expect(err).ToNot(HaveOccurred())

		workerFactory := dbng.NewWorkerFactory(dbConn)
		worker, err = workerFactory.SaveWorker(atc.Worker{
			Name:            "some-worker",
			GardenAddr:      "1.2.3.4:7777",
			BaggageclaimURL: "1.2.3.4:7788",
		}, 5*time.Minute)
		Expect(err).ToNot(HaveOccurred())

		build, err = buildFactory.CreateOneOffBuild(team)
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
		usedResourceCache, err = resourceCache.FindOrCreateForBuild(logger, setupTx, lockFactory, build)
		Expect(err).NotTo(HaveOccurred())

		Expect(setupTx.Commit()).To(Succeed())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CreateContainerVolumeWithParent", func() {
		var parentVolume dbng.CreatedVolume
		var creatingContainer *dbng.CreatingContainer

		BeforeEach(func() {
			var err error
			creatingContainer, err = containerFactory.CreateBuildContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingParentVolume, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			parentVolume, err = creatingParentVolume.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates volume for parent volume", func() {
			creatingVolume, err := volumeFactory.CreateContainerVolumeWithParent(team, worker, creatingContainer, "some-path-3", parentVolume.Handle())
			Expect(err).NotTo(HaveOccurred())

			destroyingParentVolume, err := parentVolume.Destroying()
			Expect(err).NotTo(HaveOccurred())

			_, err = destroyingParentVolume.Destroy()
			Expect(err).To(HaveOccurred())

			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume, err := createdVolume.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingVolume.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(Equal(true))

			destroyed, err = destroyingParentVolume.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(Equal(true))
		})
	})

	Describe("GetTeamVolumes", func() {
		var (
			team1handles []string
			team2handles []string
		)

		BeforeEach(func() {
			creatingContainer, err := containerFactory.CreateBuildContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			team1handles = []string{}
			team2handles = []string{}

			team2, err = teamFactory.CreateTeam("some-other-team")
			Expect(err).ToNot(HaveOccurred())

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			team1handles = append(team1handles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team2, worker, creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			team2handles = append(team2handles, createdVolume3.Handle())
		})

		It("returns only the matching team's volumes", func() {
			createdVolumes, err := volumeFactory.GetTeamVolumes(team.ID)
			Expect(err).NotTo(HaveOccurred())
			createdHandles := []string{}
			for _, vol := range createdVolumes {
				createdHandles = append(createdHandles, vol.Handle())
			}
			Expect(createdHandles).To(Equal(team1handles))

			createdVolumes2, err := volumeFactory.GetTeamVolumes(team2.ID)
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
			creatingContainer, err := containerFactory.CreateBuildContainer(worker, build, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			expectedCreatedHandles = []string{}
			expectedDestroyingHandles = []string{}

			creatingVolume1, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-1")
			Expect(err).NotTo(HaveOccurred())
			createdVolume1, err := creatingVolume1.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume1.Handle())

			creatingVolume2, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-2")
			Expect(err).NotTo(HaveOccurred())
			createdVolume2, err := creatingVolume2.Created()
			Expect(err).NotTo(HaveOccurred())
			expectedCreatedHandles = append(expectedCreatedHandles, createdVolume2.Handle())

			creatingVolume3, err := volumeFactory.CreateContainerVolume(team, worker, creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())
			createdVolume3, err := creatingVolume3.Created()
			Expect(err).NotTo(HaveOccurred())
			destroyingVolume3, err := createdVolume3.Destroying()
			Expect(err).NotTo(HaveOccurred())
			expectedDestroyingHandles = append(expectedDestroyingHandles, destroyingVolume3.Handle())

			resourceCacheVolume, err := volumeFactory.CreateResourceCacheVolume(worker, usedResourceCache)
			Expect(err).NotTo(HaveOccurred())

			_, err = resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			deleteTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			deleted, err := build.Delete(deleteTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())

			deleted, err = usedResourceCache.Destroy(deleteTx)
			Expect(err).NotTo(HaveOccurred())
			Expect(deleted).To(BeTrue())
			Expect(deleteTx.Commit()).To(Succeed())

			createdContainer, err := containerFactory.ContainerCreated(creatingContainer, "some-handle")
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
				Expect(vol.Worker().BaggageclaimURL).To(Equal("1.2.3.4:7788"))
			}
			Expect(createdHandles).To(Equal(expectedCreatedHandles))

			destoryingHandles := []string{}
			for _, vol := range destoryingVolumes {
				destoryingHandles = append(destoryingHandles, vol.Handle())
				Expect(vol.Worker().BaggageclaimURL).To(Equal("1.2.3.4:7788"))
			}
			Expect(destoryingHandles).To(Equal(destoryingHandles))
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
				volume, err := volumeFactory.CreateBaseResourceTypeVolume(team, worker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(team, worker, usedBaseResourceType)
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
				existingVolume, err = volumeFactory.CreateBaseResourceTypeVolume(team, worker, usedBaseResourceType)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindBaseResourceTypeVolume(team, worker, usedBaseResourceType)
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
			pf := dbng.NewPipelineFactory(dbConn)
			rf := dbng.NewResourceFactory(dbConn)

			pipeline, err := pf.CreatePipeline(team, "some-pipeline", "{}")
			Expect(err).ToNot(HaveOccurred())

			resource, err := rf.CreateResource(pipeline, "some-resource", "{}")
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

			usedResourceCache, err = cache.FindOrCreateForResource(logger, setupTx, lockFactory, resource)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())

		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume dbng.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeFactory.CreateResourceCacheVolume(worker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				existingVolume, err = volume.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns created volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindResourceCacheVolume(worker, usedResourceCache)
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
				existingVolume, err = volumeFactory.CreateResourceCacheVolume(worker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns creating volume", func() {
				creatingVolume, createdVolume, err := volumeFactory.FindResourceCacheVolume(worker, usedResourceCache)
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
			pf := dbng.NewPipelineFactory(dbConn)
			rf := dbng.NewResourceFactory(dbConn)

			pipeline, err := pf.CreatePipeline(team, "some-pipeline", "{}")
			Expect(err).ToNot(HaveOccurred())

			resource, err := rf.CreateResource(pipeline, "some-resource", "{}")
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

			usedResourceCache, err = cache.FindOrCreateForResource(logger, setupTx, lockFactory, resource)
			Expect(err).ToNot(HaveOccurred())

			Expect(setupTx.Commit()).To(Succeed())
		})

		Context("when there is a created volume for resource cache", func() {
			var existingVolume dbng.CreatedVolume

			BeforeEach(func() {
				var err error
				volume, err := volumeFactory.CreateResourceCacheVolume(worker, usedResourceCache)
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
					createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(worker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(createdVolume).ToNot(BeNil())
					Expect(createdVolume.Handle()).To(Equal(existingVolume.Handle()))
				})
			})

			Context("when volume is uninitialized", func() {
				It("does not return volume", func() {
					createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(worker, usedResourceCache)
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
					Expect(createdVolume).To(BeNil())
				})
			})
		})

		Context("when there is no created volume for resource cache", func() {
			It("does not return volume", func() {
				createdVolume, found, err := volumeFactory.FindResourceCacheInitializedVolume(worker, usedResourceCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(createdVolume).To(BeNil())
			})
		})
	})
})
