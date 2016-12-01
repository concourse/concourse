package dbng_test

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Volume", func() {
	Describe("creatingVolume.Created", func() {
		var (
			creatingVolume dbng.CreatingVolume
			createdVolume  dbng.CreatedVolume
		)

		BeforeEach(func() {
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam, defaultWorker, deafultCreatingContainer, "/path/to/volume")
		})

		JustBeforeEach(func() {
			createdVolume, err = creatingVolume.Created()
		})

		Describe("the database query fails", func() {
			Context("when the volume has exited the `creating` state", func() {
				BeforeEach(func() {
					_, err = creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the correct error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(dbng.ErrVolumeMarkCreatedFailed))
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
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(dbng.ErrVolumeMarkCreatedFailed))
				})
			})
		})

		Describe("the database query succeeds", func() {
			It("updates the record to be `created`", func() {
				foundVolumes, err := volumeFactory.FindVolumesForContainer(defaultCreatedContainer.ID)
				Expect(err).NotTo(HaveOccurred())
				Expect(foundVolumes).To(ContainElement(WithTransform(dbng.CreatedVolume.Path, Equal("/path/to/volume"))))
			})

			It("returns a createdVolume and no error", func() {
				Expect(createdVolume).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("createdVolume.Initialize", func() {
		var createdVolume dbng.CreatedVolume

		BeforeEach(func() {
			setupTx, err := dbConn.Begin()
			Expect(err).ToNot(HaveOccurred())
			resourceType := atc.ResourceType{
				Name: "some-type",
				Type: "some-base-resource-type",
				Source: atc.Source{
					"some-type": "source",
				},
			}
			_, err = dbng.ResourceType{
				ResourceType: resourceType,
				Pipeline:     defaultPipeline,
			}.Create(setupTx, atc.Version{"some-type": "version"})
			Expect(err).NotTo(HaveOccurred())
			Expect(setupTx.Commit()).To(Succeed())

			resourceCache, err := resourceCacheFactory.FindOrCreateResourceCacheForBuild(
				logger,
				defaultBuild,
				"some-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": "params"},
				defaultPipeline,
				atc.ResourceTypes{
					resourceType,
				},
			)

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

	Describe("createdVolume.CreateChildForContainer", func() { // TODO TESTME when cow is a thing
		var parentVolume dbng.CreatedVolume
		var creatingContainer *dbng.CreatingContainer

		BeforeEach(func() {
			var err error
			creatingContainer, err = containerFactory.CreateBuildContainer(defaultWorker, defaultBuild, "some-plan", dbng.ContainerMetadata{
				Type: "task",
				Name: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			creatingParentVolume, err := volumeFactory.CreateContainerVolume(defaultTeam, defaultWorker, creatingContainer, "some-path-1")
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
	})

	Describe("createdVolume.Destroying", func() {
		var (
			createdVolume    dbng.CreatedVolume
			destroyingVolume dbng.DestroyingVolume
		)

		BeforeEach(func() {
			var creatingVolume dbng.CreatingVolume
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam, defaultWorker, deafultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err = creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			destroyingVolume, err = createdVolume.Destroying()
		})

		Describe("the database query fails", func() {
			Context("when the volume has exited the `created` state", func() {
				BeforeEach(func() {
					_, err = createdVolume.Destroying()
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the correct error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(dbng.ErrVolumeMarkDestroyingFailed))
				})
			})

			Context("there is no such id in the table", func() {
				BeforeEach(func() {
					vd, err := createdVolume.Destroying()
					Expect(err).NotTo(HaveOccurred())

					deleted, err := vd.Destroy()
					Expect(err).NotTo(HaveOccurred())
					Expect(deleted).To(BeTrue())
				})

				It("returns the correct error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(dbng.ErrVolumeMarkDestroyingFailed))
				})
			})
		})

		Describe("the database query succeeds", func() {
			BeforeEach(func() {
				destroyingContainer, err := defaultCreatedContainer.Destroying()
				Expect(err).NotTo(HaveOccurred())

				didDestroy, err := destroyingContainer.Destroy()
				Expect(err).NotTo(HaveOccurred())
				Expect(didDestroy).To(BeTrue())
			})

			It("updates the record to be `destroying`", func() {
				_, destroying, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())

				found := false
				for _, d := range destroying {
					if d.Handle() == destroyingVolume.Handle() {
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})

			It("returns a createdVolume and no error", func() {
				Expect(destroyingVolume).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("destroyingVolume.Destroy", func() {
		var (
			destroyingVolume dbng.DestroyingVolume
			didDestroy       bool
		)

		BeforeEach(func() {
			var creatingVolume dbng.CreatingVolume
			var createdVolume dbng.CreatedVolume
			creatingVolume, err = volumeFactory.CreateContainerVolume(defaultTeam, defaultWorker, deafultCreatingContainer, "/path/to/volume")
			Expect(err).NotTo(HaveOccurred())

			createdVolume, err = creatingVolume.Created()
			Expect(err).NotTo(HaveOccurred())

			destroyingVolume, err = createdVolume.Destroying()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			didDestroy, err = destroyingVolume.Destroy()
		})

		Context("when the volume has already been removed", func() {
			BeforeEach(func() {
				didDestroy, err = destroyingVolume.Destroy()
				Expect(err).NotTo(HaveOccurred())
				Expect(didDestroy).To(BeTrue())
			})

			It("returns false and no error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(didDestroy).To(BeFalse())
			})
		})

		Describe("the database query succeeds", func() {
			BeforeEach(func() {
				destroyingContainer, err := defaultCreatedContainer.Destroying()
				Expect(err).NotTo(HaveOccurred())

				didDestroy, err := destroyingContainer.Destroy()
				Expect(err).NotTo(HaveOccurred())
				Expect(didDestroy).To(BeTrue())
			})

			It("removes the record from the DB", func() {
				_, destroying, err := volumeFactory.GetOrphanedVolumes()
				Expect(err).NotTo(HaveOccurred())

				found := false
				for _, d := range destroying {
					if d.Handle() == destroyingVolume.Handle() {
						found = true
					}
				}
				Expect(found).To(BeFalse())
			})

			It("returns true and no error", func() {
				Expect(didDestroy).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
