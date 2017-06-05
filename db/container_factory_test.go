package db_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerFactory", func() {
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

	Describe("FindContainersForDeletion", func() {
		Describe("task containers", func() {
			var (
				creatingContainer db.CreatingContainer
				build             db.Build
			)

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the build is finded as interceptible", func() {
				BeforeEach(func() {
					err := build.SetInterceptible(true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the build is marked as non-interceptible", func() {
				BeforeEach(func() {
					err := build.SetInterceptible(false)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when the container is creating", func() {
					It("finds container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(HaveLen(1))
						Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
						Expect(createdContainers).To(BeEmpty())
						Expect(destroyingContainers).To(BeEmpty())
					})
				})

				Context("when the container is created", func() {
					BeforeEach(func() {
						_, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(HaveLen(1))
						Expect(createdContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
						Expect(destroyingContainers).To(BeEmpty())
					})
				})

				Context("when the container is destroying", func() {
					BeforeEach(func() {
						createdContainer, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
						_, err = createdContainer.Destroying()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(BeEmpty())
						Expect(destroyingContainers).To(HaveLen(1))
						Expect(destroyingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					})
				})
			})

			Context("when build is deleted", func() {
				BeforeEach(func() {
					err := defaultPipeline.Destroy()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		Describe("check containers", func() {
			var (
				creatingContainer db.CreatingContainer
				resourceConfig    *db.UsedResourceConfig
			)

			BeforeEach(func() {
				var err error
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
					logger,
					db.ForResource(defaultResource.ID()),
					"some-base-resource-type",
					atc.Source{"some": "source"},
					atc.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig, fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when check container best if use by date is expired", func() {
				BeforeEach(func() {
					_, err := psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": creatingContainer.ID()}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when container is creating", func() {
					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(HaveLen(1))
						Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
						Expect(createdContainers).To(BeEmpty())
						Expect(destroyingContainers).To(BeEmpty())
					})
				})

				Context("when container is created", func() {
					BeforeEach(func() {
						_, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(HaveLen(1))
						Expect(createdContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
						Expect(destroyingContainers).To(BeEmpty())
					})
				})

				Context("when container is destroying", func() {
					BeforeEach(func() {
						createdContainer, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
						_, err = createdContainer.Destroying()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(BeEmpty())
						Expect(destroyingContainers).To(HaveLen(1))
						Expect(destroyingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					})
				})
			})

			Context("when check container best if use by date did not expire", func() {
				BeforeEach(func() {
					_, err := psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() + '1 hour'::INTERVAL")).
						Where(sq.Eq{"id": creatingContainer.ID()}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the resource config is deleted", func() {
				BeforeEach(func() {
					err := defaultPipeline.Destroy()
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfigFactory.CleanConfigUsesForInactiveResources()
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfigFactory.CleanUselessConfigs()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())
					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(HaveLen(1))
					Expect(createdContainers[0].Handle()).To(Equal(defaultCreatedContainer.Handle()))
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the worker base resource type has a new version", func() {
				BeforeEach(func() {
					var err error
					newlyUpdatedWorker := defaultWorkerPayload
					newlyUpdatedResource := defaultWorkerPayload.ResourceTypes[0]
					newlyUpdatedResource.Version = newlyUpdatedResource.Version + "-new"
					newlyUpdatedWorker.ResourceTypes = []atc.WorkerResourceType{newlyUpdatedResource}

					defaultWorker, err = workerFactory.SaveWorker(newlyUpdatedWorker, 0)
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())
					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(HaveLen(1))
					Expect(createdContainers[0].Handle()).To(Equal(defaultCreatedContainer.Handle()))
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the same worker base resource type is saved", func() {
				BeforeEach(func() {
					var err error
					sameWorker := defaultWorkerPayload

					defaultWorker, err = workerFactory.SaveWorker(sameWorker, 0)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		Describe("get containers", func() {
			var (
				creatingContainer db.CreatingContainer
				resourceCache     *db.UsedResourceCache
			)

			BeforeEach(func() {
				var err error
				resourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					db.ForResource(defaultResource.ID()),
					"some-base-resource-type",
					atc.Version{"some": "version"},
					atc.Source{"some": "source"},
					atc.Params{},
					atc.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceGetContainer(defaultWorker.Name(), resourceCache, fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the resource cache is deleted", func() {
				BeforeEach(func() {
					err := defaultPipeline.Destroy()
					Expect(err).NotTo(HaveOccurred())

					err = resourceCacheFactory.CleanUsesForInactiveResources()
					Expect(err).NotTo(HaveOccurred())

					err = resourceCacheFactory.CleanUpInvalidCaches()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})

				Context("when container is created", func() {
					BeforeEach(func() {
						_, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(HaveLen(1))
						Expect(createdContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
						Expect(destroyingContainers).To(BeEmpty())
					})
				})

				Context("when container is destroying", func() {
					BeforeEach(func() {
						createdContainer, err := creatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
						_, err = createdContainer.Destroying()
						Expect(err).NotTo(HaveOccurred())
					})

					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(creatingContainers).To(BeEmpty())
						Expect(createdContainers).To(BeEmpty())
						Expect(destroyingContainers).To(HaveLen(1))
						Expect(destroyingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					})
				})
			})

			Context("when volume for resource cache is initialized", func() {
				BeforeEach(func() {
					creatingVolume, err := volumeFactory.CreateResourceCacheVolume(defaultWorker, resourceCache)
					Expect(err).NotTo(HaveOccurred())
					createdVolume, err := creatingVolume.Created()
					Expect(err).NotTo(HaveOccurred())
					err = createdVolume.Initialize()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when there are no uses for resource cache", func() {
				BeforeEach(func() {
					err := defaultPipeline.Destroy()
					Expect(err).NotTo(HaveOccurred())

					err = resourceCacheFactory.CleanUsesForInactiveResources()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		FDescribe("containers for creating containers", func() {
			var (
				creatingTaskContainer db.CreatingContainer
				creatingContainer     db.CreatingContainer
				build                 db.Build
			)

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingTaskContainer, err = defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), fullMetadata)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewCreatingContainerContainerOwner(creatingTaskContainer), fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the container they're for is still creating", func() {
				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the container they're for is created", func() {
				BeforeEach(func() {
					_, err := creatingTaskContainer.Created()
					Expect(err).ToNot(HaveOccurred())
				})

				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when the container they're for is gone", func() {
				BeforeEach(func() {
					_, err := creatingTaskContainer.Created()
					Expect(err).ToNot(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerFactory.FindContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})
	})
})
