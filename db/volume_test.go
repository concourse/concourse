package db_test

import (
	"time"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume", func() {
	var defaultCreatingContainer db.CreatingContainer
	var defaultCreatedContainer db.CreatedContainer

	BeforeEach(func() {
		expiries := db.ContainerOwnerExpiries{
			GraceTime: 2 * time.Minute,
			Min:       5 * time.Minute,
			Max:       1 * time.Hour,
		}

		resourceConfigCheckSession, err := resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(logger, "some-base-resource-type", atc.Source{}, creds.VersionedResourceTypes{}, expiries)
		Expect(err).NotTo(HaveOccurred())

		defaultCreatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID()), db.ContainerMetadata{Type: "check"})
		Expect(err).NotTo(HaveOccurred())

		defaultCreatedContainer, err = defaultCreatingContainer.Created()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("creatingVolume.Failed", func() {
		var (
			creatingVolume db.CreatingVolume
			failedVolume   db.FailedVolume
			failErr        error
		)

		BeforeEach(func() {
			var err error
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
			Expect(err).ToNot(HaveOccurred())
		})

		JustBeforeEach(func() {
			failedVolume, failErr = creatingVolume.Failed()
		})

		Describe("the database query fails", func() {
			Context("when the volume is not in creating or failed state", func() {
				BeforeEach(func() {
					_, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the correct error", func() {
					Expect(failErr).To(HaveOccurred())
					Expect(failErr).To(Equal(db.ErrVolumeMarkStateFailed{db.VolumeStateFailed}))
				})
			})

			Context("there is no such id in the table", func() {
				BeforeEach(func() {
					createdVol, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())

					destroyingVol, err := createdVol.Destroying()
					Expect(err).NotTo(HaveOccurred())

					deleted, err := destroyingVol.Destroy()
					Expect(err).NotTo(HaveOccurred())
					Expect(deleted).To(BeTrue())
				})

				It("returns the correct error", func() {
					Expect(failErr).To(HaveOccurred())
					Expect(failErr).To(Equal(db.ErrVolumeMarkStateFailed{db.VolumeStateFailed}))
				})
			})
		})

		Describe("the database query succeeds", func() {
			It("updates the record to be `failed`", func() {
				Expect(failErr).ToNot(HaveOccurred())

				failedVolumes, err := volumeFactory.GetFailedVolumes()
				Expect(err).NotTo(HaveOccurred())

				Expect(failedVolumes).To(HaveLen(1))
				Expect(failedVolumes).To(ContainElement(failedVolume))
			})

			Context("when the volume is already in the failed state", func() {
				BeforeEach(func() {
					creatingVolume.Failed()
				})

				It("does not fail to transition", func() {
					Expect(failErr).ToNot(HaveOccurred())
				})
			})

		})
	})

	Describe("creatingVolume.Created", func() {
		var (
			creatingVolume db.CreatingVolume
			createdVolume  db.CreatedVolume
			createErr      error
		)

		BeforeEach(func() {
			var err error
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
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

	Describe("createdVolume.InitializeResourceCache", func() {
		var createdVolume db.CreatedVolume
		var resourceCache *db.UsedResourceCache
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			resourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
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
				),
			)
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "get",
				StepName: "some-resource",
			})
			Expect(err).ToNot(HaveOccurred())

			resourceCacheVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path")
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err = resourceCacheVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			err = createdVolume.InitializeResourceCache(resourceCache)
			Expect(err).NotTo(HaveOccurred())
		})

		It("associates the volume to the resource cache", func() {
			foundVolume, found, err := volumeFactory.FindResourceCacheVolume(defaultWorker.Name(), resourceCache)
			Expect(err).NotTo(HaveOccurred())
			Expect(foundVolume.Handle()).To(Equal(createdVolume.Handle()))
			Expect(found).To(BeTrue())
		})

		Context("when there's already an initialized resource cache on the same worker", func() {
			It("leaves the volume owned by the the container", func() {
				creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
					Type:     "get",
					StepName: "some-resource",
				})
				Expect(err).ToNot(HaveOccurred())

				resourceCacheVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path")
				Expect(err).NotTo(HaveOccurred())

				createdVolume, err = resourceCacheVolume.Created()
				Expect(err).NotTo(HaveOccurred())

				err = createdVolume.InitializeResourceCache(resourceCache)
				Expect(err).NotTo(HaveOccurred())

				Expect(createdVolume.Type()).To(Equal(db.VolumeTypeContainer))
			})
		})
	})

	Describe("createdVolume.InitializeTaskCache", func() {
		Context("when there is a volume that belongs to worker task cache", func() {
			var (
				existingTaskCacheVolume db.CreatedVolume
				volume                  db.CreatedVolume
			)

			BeforeEach(func() {
				build, err := defaultTeam.CreateOneOffBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{})
				Expect(err).ToNot(HaveOccurred())

				v, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path")
				Expect(err).NotTo(HaveOccurred())

				existingTaskCacheVolume, err = v.Created()
				Expect(err).NotTo(HaveOccurred())

				err = existingTaskCacheVolume.InitializeTaskCache(defaultJob.ID(), "some-step", "some-cache-path")
				Expect(err).NotTo(HaveOccurred())

				v, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-other-path")
				Expect(err).NotTo(HaveOccurred())

				volume, err = v.Created()
				Expect(err).NotTo(HaveOccurred())
			})

			It("sets current volume as worker task cache volume", func() {
				uwtc, err := workerTaskCacheFactory.FindOrCreate(defaultJob.ID(), "some-step", "some-cache-path", defaultWorker.Name())
				Expect(err).NotTo(HaveOccurred())

				creatingVolume, createdVolume, err := volumeFactory.FindTaskCacheVolume(defaultTeam.ID(), uwtc)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).To(BeNil())
				Expect(createdVolume).NotTo(BeNil())
				Expect(createdVolume.Handle()).To(Equal(existingTaskCacheVolume.Handle()))

				err = volume.InitializeTaskCache(defaultJob.ID(), "some-step", "some-cache-path")
				Expect(err).NotTo(HaveOccurred())

				creatingVolume, createdVolume, err = volumeFactory.FindTaskCacheVolume(defaultTeam.ID(), uwtc)
				Expect(err).NotTo(HaveOccurred())
				Expect(creatingVolume).To(BeNil())
				Expect(createdVolume).NotTo(BeNil())
				Expect(createdVolume.Handle()).To(Equal(volume.Handle()))

				Expect(existingTaskCacheVolume.Handle()).NotTo(Equal(volume.Handle()))
			})
		})
	})

	Describe("Container volumes", func() {
		It("returns volume type, container handle, mount path", func() {
			creatingVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(createdVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(createdVolume.Path()).To(Equal("/path/to/volume"))

			_, createdVolume, err = volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(createdVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(createdVolume.Path()).To(Equal("/path/to/volume"))
		})
	})

	Describe("Volumes created from a parent", func() {
		It("returns parent handle", func() {
			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
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

			_, childVolume, err = volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/child/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(childVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))
			Expect(childVolume.ContainerHandle()).To(Equal(defaultCreatingContainer.Handle()))
			Expect(childVolume.Path()).To(Equal("/path/to/child/volume"))
			Expect(childVolume.ParentHandle()).To(Equal(createdParentVolume.Handle()))
		})

		It("prevents the parent from being destroyed", func() {
			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
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

	Describe("Resource cache volumes", func() {
		It("returns volume type, resource type, resource version", func() {
			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name:   "some-type",
								Type:   "some-base-resource-type",
								Source: atc.Source{"some-type": "((source-param))"},
							},
							Version: atc.Version{"some-custom-type": "version"},
						},
					},
				),
			)
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "get",
				StepName: "some-resource",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path")
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeContainer)))

			err = createdVolume.InitializeResourceCache(resourceCache)
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResource)))

			volumeResourceType, err := createdVolume.ResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Version).To(Equal("some-brt-version"))
			Expect(volumeResourceType.ResourceType.Version).To(Equal(atc.Version{"some-custom-type": "version"}))
			Expect(volumeResourceType.Version).To(Equal(atc.Version{"some": "version"}))

			createdVolume, found, err := volumeFactory.FindResourceCacheVolume(defaultWorker.Name(), resourceCache)
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(createdVolume.Type()).To(Equal(db.VolumeType(db.VolumeTypeResource)))
			volumeResourceType, err = createdVolume.ResourceType()
			Expect(err).NotTo(HaveOccurred())
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Name).To(Equal("some-base-resource-type"))
			Expect(volumeResourceType.ResourceType.WorkerBaseResourceType.Version).To(Equal("some-brt-version"))
			Expect(volumeResourceType.ResourceType.Version).To(Equal(atc.Version{"some-custom-type": "version"}))
			Expect(volumeResourceType.Version).To(Equal(atc.Version{"some": "version"}))
		})
	})

	Describe("Resource type volumes", func() {
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

	Describe("Task cache volumes", func() {
		It("returns volume type and task identifier", func() {
			uwtc, err := workerTaskCacheFactory.FindOrCreate(defaultJob.ID(), "some-task", "some-path", defaultWorker.Name())
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, err := volumeFactory.CreateTaskCacheVolume(defaultTeam.ID(), uwtc)
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err := creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			Expect(createdVolume.Type()).To(Equal(db.VolumeTypeTaskCache))

			pipelineName, jobName, stepName, err := createdVolume.TaskIdentifier()
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineName).To(Equal(defaultPipeline.Name()))
			Expect(jobName).To(Equal(defaultJob.Name()))
			Expect(stepName).To(Equal("some-task"))
		})
	})

	Describe("createdVolume.CreateChildForContainer", func() {
		var parentVolume db.CreatedVolume
		var creatingContainer db.CreatingContainer

		BeforeEach(func() {
			build, err := defaultTeam.CreateOneOffBuild()
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			usedResourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
				logger,
				db.ForBuild(build.ID()),
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{"some": "source"},
				atc.Params{"some": "params"},
				creds.NewVersionedResourceTypes(template.StaticVariables{"source-param": "some-secret-sauce"},
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
				),
			)
			Expect(err).NotTo(HaveOccurred())

			creatingContainer, err := defaultTeam.CreateContainer(defaultWorker.Name(), db.NewBuildStepContainerOwner(build.ID(), "some-plan"), db.ContainerMetadata{
				Type:     "get",
				StepName: "some-resource",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), creatingContainer, "some-path")
			Expect(err).NotTo(HaveOccurred())

			parentVolume, err = creatingParentVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			err = parentVolume.InitializeResourceCache(usedResourceCache)
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
	})

	Context("when worker is no longer in database", func() {
		var creatingVolume db.CreatingVolume

		BeforeEach(func() {
			var err error
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
			Expect(err).ToNot(HaveOccurred())
		})

		It("the container goes away from the db", func() {
			err := defaultWorker.Delete()
			Expect(err).NotTo(HaveOccurred())

			creatingVolume, createdVolume, err := volumeFactory.FindContainerVolume(defaultTeam.ID(), defaultWorker.Name(), defaultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())
			Expect(creatingVolume).To(BeNil())
			Expect(createdVolume).To(BeNil())
		})
	})
})
