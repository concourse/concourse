package db_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume", func() {
	var defaultCreatingContainer db.CreatingContainer
	var defaultCreatedContainer db.CreatedContainer

	BeforeEach(func() {
		config, err := resourceConfigFactory.FindOrCreateResourceConfig(logger, db.ForResource(defaultResource.ID()), "some-base-resource-type", atc.Source{}, atc.VersionedResourceTypes{})
		Expect(err).NotTo(HaveOccurred())

		defaultCreatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), config, db.ContainerMetadata{Type: "check"})
		Expect(err).NotTo(HaveOccurred())

		defaultCreatedContainer, err = defaultCreatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("creatingVolume.Created", func() {
		var (
			creatingVolume db.CreatingVolume
			createdVolume  db.CreatedVolume
			createErr      error
		)

		BeforeEach(func() {
			var err error
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			createdVolume, createErr = creatingVolume.Created()
		})

		Describe("the database query fails", func() {
			Context("when the volume is not in creating or created state", func() {
				BeforeEach(func() {
					createdVolume, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())
					_, err = createdVolume.Destroying()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the correct error", func() {
					Expect(createErr).To(HaveOccurred())
					Expect(createErr).To(Equal(db.ErrVolumeMarkCreatedFailed{Handle: creatingVolume.Handle()}))
				})
			})

			Context("there is no such id in the table", func() {
				BeforeEach(func() {
					vc, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())

					vd, err := vc.Destroying()
					Expect(err).NotTo(HaveOccurred())

					deleted, err := vd.Destroy()
					Expect(err).NotTo(HaveOccurred())
					Expect(deleted).To(BeTrue())
				})

				It("returns the correct error", func() {
					Expect(createErr).To(HaveOccurred())
					Expect(createErr).To(Equal(db.ErrVolumeMarkCreatedFailed{Handle: creatingVolume.Handle()}))
				})
			})
		})

		Describe("the database query succeeds", func() {
			It("updates the record to be `created`", func() {
				foundVolumes, err := volumeFactory.FindVolumesForContainer(defaultCreatedContainer)
				Expect(err).NotTo(HaveOccurred())
				Expect(foundVolumes).To(ContainElement(WithTransform(db.CreatedVolume.Path, Equal("/path/to/volume"))))
			})

			It("returns a createdVolume and no error", func() {
				Expect(createdVolume).NotTo(BeNil())
				Expect(createErr).NotTo(HaveOccurred())
			})

			Context("when volume is already in provided state", func() {
				BeforeEach(func() {
					_, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns a createdVolume and no error", func() {
					Expect(createdVolume).NotTo(BeNil())
					Expect(createErr).NotTo(HaveOccurred())
				})
			})
		})
	})

	Describe("createdVolume.Initialize", func() {
		var createdVolume db.CreatedVolume
		var defaultBuild db.Build

		BeforeEach(func() {
			var err error
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(defaultBuild.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name:   "some-type",
							Type:   "some-base-resource-type",
							Source: atc.Source{"some-type": "source"},
						},
						Version: atc.Version{"some-type": "version"},
					},
				},
			)
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, resourceCache)
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err = creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		It("sets initialized", func() {
			Expect(createdVolume.IsInitialized()).To(BeFalse())
			err := createdVolume.Initialize()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.IsInitialized()).To(BeTrue())
		})
	})

	Context("when volume type is VolumeTypeContainer", func() {
		It("returns volume type, container handle, mount path", func() {
			creatingVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(createdVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(createdVolume.Path()).To(Equal("/path/to/volume"))

			_, createdVolume, err = volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(createdVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(createdVolume.Path()).To(Equal("/path/to/volume"))
		})
	})

	Context("when volume has parent", func() {
		It("returns parent handle", func() {
			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			createdParentVolume, err := creatingParentVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			childCreatingVolume, err := createdParentVolume.CreateChildForContainer(defaultCreatingContainer, "/path/to/child/volume")
			Expect(err).NotTo(HaveOccurred())
			childVolume, err := childCreatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(childVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(childVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(childVolume.Path()).To(Equal("/path/to/child/volume"))
			Expect(childVolume.ParentHandle()).To(Equal(createdParentVolume.Handle()))

			_, childVolume, err = volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/child/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(childVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(childVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(childVolume.Path()).To(Equal("/path/to/child/volume"))
			Expect(childVolume.ParentHandle()).To(Equal(createdParentVolume.Handle()))
		})

		It("prevents the parent from being destroyed", func() {
			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			createdParentVolume, err := creatingParentVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			childCreatingVolume, err := createdParentVolume.CreateChildForContainer(defaultCreatingContainer, "/path/to/child/volume")
			Expect(err).NotTo(HaveOccurred())
			_, err = childCreatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			_, err = createdParentVolume.Destroying()
			Expect(err).To(Equal(db.ErrVolumeCannotBeDestroyedWithChildrenPresent))
		})
	})

	Context("when volume type is VolumeTypeResource", func() {
		It("returns volume type, resource type, resource version", func() {
			defaultBuild, err := defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(defaultBuild.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{"some": "params"},
				atc.VersionedResourceTypes{
					{
						ResourceType: atc.ResourceType{
							Name:   "some-type",
							Type:   "some-base-resource-type",
							Source: atc.Source{"some-type": "source"},
						},
						Version: atc.Version{"some-custom-type": "version"},
					},
				},
			)
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, resourceCache)
			Expect(err).NotTo(HaveOccurred())
			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResource)))

			volumeResourceType, err := createdVolume.ResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Version).To(Equal("some-brt-version"))
			Expect(volumeResourceType.ResourceType.Version).To(Equal(atc.Version{"some-custom-type": "version"}))
			Expect(volumeResourceType.Version).To(Equal(atc.Version{"some": "version"}))

			_, createdVolume, err = volumeFactory.FindResourceCacheVolume(defaultWorker, resourceCache)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResource)))
			volumeResourceType, err = createdVolume.ResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Version).To(Equal("some-brt-version"))
			Expect(volumeResourceType.ResourceType.Version).To(Equal(atc.Version{"some-custom-type": "version"}))
			Expect(volumeResourceType.Version).To(Equal(atc.Version{"some": "version"}))
		})
	})

	Context("when volume type is VolumeTypeResourceType", func() {
		It("returns volume type, base resource type name, base resource type version", func() {
			usedWorkerBaseResourceType, found, err := workerBaseResourceTypeFactory.Find("some-base-resource-type", defaultWorker)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			creatingVolume, err := volumeFactory.CreateBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
			Expect(err).NotTo(HaveOccurred())
			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResourceType)))
			volumeBaseResourceType, err := createdVolume.BaseResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeBaseResourceType.Version).To(Equal("some-brt-version"))

			_, createdVolume, err = volumeFactory.FindBaseResourceTypeVolume(defaultTeam.ID(), usedWorkerBaseResourceType)
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResourceType)))
			volumeBaseResourceType, err = createdVolume.BaseResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeBaseResourceType.Version).To(Equal("some-brt-version"))
		})
	})

	Describe("createdVolume.CreateChildForContainer", func() {
		var parentVolume db.CreatedVolume
		var creatingContainer db.CreatingContainer
		var defaultBuild db.Build

		BeforeEach(func() {
			var err error
			defaultBuild, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())
			creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(defaultBuild.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			defer setupTx.Rollback()

			resourceCache := db.ResourceCache{
				ResourceConfig: db.ResourceConfig{
					CreatedByBaseResourceType: &db.BaseResourceType{
						Name: "some-base-resource-type",
					},
				},
			}
			usedResourceCache, err := db.ForBuild(defaultBuild.ID()).UseResourceCache(logger, setupTx, lockFactory, resourceCache)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			creatingParentVolume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, usedResourceCache)
			Expect(err).NotTo(HaveOccurred())

			parentVolume, err = creatingParentVolume.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		It("creates volume for parent volume", func() {
			creatingChildVolume, err := parentVolume.CreateChildForContainer(creatingContainer, "some-path-3")
			Expect(err).NotTo(HaveOccurred())

			_, err = parentVolume.Destroying()
			Expect(err).To(HaveOccurred())

			createdChildVolume, err := creatingChildVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			destroyingChildVolume, err := createdChildVolume.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err := destroyingChildVolume.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(Equal(true))

			destroyingParentVolume, err := parentVolume.Destroying()
			Expect(err).NotTo(HaveOccurred())
			destroyed, err = destroyingParentVolume.Destroy()
			Expect(err).NotTo(HaveOccurred())
			Expect(destroyed).To(Equal(true))
		})

		Context("when parent volume is initialized", func() {
			It("creates intiialized volume", func() {
				err := parentVolume.Initialize()
				Expect(err).NotTo(HaveOccurred())

				creatingChildVolume, err := parentVolume.CreateChildForContainer(creatingContainer, "some-path-3")
				Expect(err).NotTo(HaveOccurred())

				createdChildVolume, err := creatingChildVolume.Created()
				Expect(err).NotTo(HaveOccurred())

				Expect(createdChildVolume.IsInitialized()).To(BeTrue())
			})
		})
	})

	Context("when worker is no longer in database", func() {
		var creatingVolume db.CreatingVolume

		BeforeEach(func() {
			var err error
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).ToNot(HaveOccurred())
		})

		It("the container goes away from the db", func() {
			err := defaultWorker.Delete()
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, createdVolume, err := volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker, defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(creatingVolume).To(BeNil())
			Expect(createdVolume).To(BeNil())
		})
	})
})
