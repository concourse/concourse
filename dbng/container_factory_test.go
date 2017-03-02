package dbng_test

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerFactory", func() {
	Describe("FindContainersForDeletion", func() {
		Describe("task containers", func() {
			var (
				creatingContainer dbng.CreatingContainer
				build             dbng.Build
			)

			BeforeEach(func() {
				build, err = defaultPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker.Name(), build.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the build is finded as interceptible", func() {
				BeforeEach(func() {
					err = build.SetInterceptible(true)
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
					err = build.SetInterceptible(false)
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
				creatingContainer dbng.CreatingContainer
				resourceConfig    *dbng.UsedResourceConfig
			)

			BeforeEach(func() {
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfigForResource(
					logger,
					defaultResource.ID,
					"some-base-resource-type",
					atc.Source{"some": "source"},
					defaultPipeline.ID(),
					atc.ResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when check container best if use by date is expired", func() {
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					_, err = psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": creatingContainer.ID()}).
						RunWith(tx).Exec()
					Expect(err).NotTo(HaveOccurred())

					Expect(tx.Commit()).To(Succeed())
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
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					_, err = psql.Update("containers").
						Set("best_if_used_by", sq.Expr("NOW() + '1 hour'::INTERVAL")).
						Where(sq.Eq{"id": creatingContainer.ID()}).
						RunWith(tx).Exec()
					Expect(err).NotTo(HaveOccurred())

					Expect(tx.Commit()).To(Succeed())
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
				creatingContainer dbng.CreatingContainer
				resourceCache     *dbng.UsedResourceCache
			)

			BeforeEach(func() {
				resourceCache, err = resourceCacheFactory.FindOrCreateResourceCacheForResource(
					logger,
					defaultResource.ID,
					"some-base-resource-type",
					atc.Version{"some": "version"},
					atc.Source{"some": "source"},
					atc.Params{},
					defaultPipeline.ID(),
					atc.ResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceGetContainer(defaultWorker.Name(), resourceCache, "some-task")
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
						_, err = creatingContainer.Created()
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
	})
})
