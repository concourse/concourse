package dbng_test

import (
	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerFactory", func() {
	Describe("FindContainersMarkedForDeletion", func() {
		It("does not find non-deleting containers", func() {
			deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
			Expect(err).NotTo(HaveOccurred())

			Expect(deletingContainers).To(BeEmpty())
		})

		It("does find deleting containers", func() {
			destroyingContainer, err := defaultCreatedContainer.Destroying()
			Expect(err).NotTo(HaveOccurred())

			deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
			Expect(err).NotTo(HaveOccurred())

			Expect(deletingContainers).To(HaveLen(1))

			destroyedContainer := deletingContainers[0]
			Expect(destroyedContainer.Handle()).To(Equal(destroyingContainer.Handle()))
			Expect(destroyedContainer.WorkerName()).To(Equal(destroyingContainer.WorkerName()))
		})
	})

	Describe("MarkContainersForDeletion", func() {

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

			Context("when the container is creating", func() {
				It("does not mark the container for deletion", func() {
					err = containerFactory.MarkContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(deletingContainers).To(BeEmpty())
				})
			})

			Context("when the container is created", func() {
				var createdContainer dbng.CreatedContainer

				BeforeEach(func() {
					createdContainer, err = creatingContainer.Created()
				})

				Context("when the build is marked as interceptible", func() {
					BeforeEach(func() {
						err = build.SetInterceptible(true)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not mark container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(BeEmpty())
					})
				})

				Context("when the build is marked as non-interceptible", func() {
					BeforeEach(func() {
						err = build.SetInterceptible(false)
						Expect(err).NotTo(HaveOccurred())
					})

					It("marks container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(1))
						Expect(deletingContainers[0].Handle()).To(Equal(createdContainer.Handle()))
					})
				})

				Context("when build is deleted", func() {
					BeforeEach(func() {
						err := defaultPipeline.Destroy()
						Expect(err).NotTo(HaveOccurred())
					})

					It("marks container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(1))
						Expect(deletingContainers[0].Handle()).To(Equal(createdContainer.Handle()))
					})
				})
			})
		})

		Describe("check containers", func() {
			var (
				creatingContainer dbng.CreatingContainer
				resourceConfig    *dbng.UsedResourceConfig
			)

			BeforeEach(func() {
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
					logger,
					dbng.ForResource{defaultResource.ID},
					"some-base-resource-type",
					atc.Source{"some": "source"},
					defaultPipeline.ID(),
					atc.ResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker.Name(), resourceConfig)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the container is creating", func() {
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

				It("does not mark the container for deletion", func() {
					err = containerFactory.MarkContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(deletingContainers).To(BeEmpty())
				})
			})

			Context("when container is created", func() {
				var createdContainer dbng.CreatedContainer

				BeforeEach(func() {
					createdContainer, err = creatingContainer.Created()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when check container best if use by date is expired", func() {
					BeforeEach(func() {
						tx, err := dbConn.Begin()
						Expect(err).NotTo(HaveOccurred())

						_, err = psql.Update("containers").
							Set("best_if_used_by", sq.Expr("NOW() - '1 second'::INTERVAL")).
							Where(sq.Eq{"id": createdContainer.ID()}).
							RunWith(tx).Exec()
						Expect(err).NotTo(HaveOccurred())

						Expect(tx.Commit()).To(Succeed())
					})

					It("marks the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(1))
					})
				})

				Context("when check container best if use by date did not expire", func() {
					BeforeEach(func() {
						tx, err := dbConn.Begin()
						Expect(err).NotTo(HaveOccurred())

						_, err = psql.Update("containers").
							Set("best_if_used_by", sq.Expr("NOW() + '1 hour'::INTERVAL")).
							Where(sq.Eq{"id": createdContainer.ID()}).
							RunWith(tx).Exec()
						Expect(err).NotTo(HaveOccurred())

						Expect(tx.Commit()).To(Succeed())
					})

					It("does not mark the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(BeEmpty())
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

					It("marks the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(2))
						Expect([]string{
							deletingContainers[0].Handle(),
							deletingContainers[1].Handle(),
						}).To(ConsistOf([]string{
							defaultCreatedContainer.Handle(),
							createdContainer.Handle(),
						}))
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

					It("marks the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(2))
						Expect([]string{
							deletingContainers[0].Handle(),
							deletingContainers[1].Handle(),
						}).To(ConsistOf([]string{
							defaultCreatedContainer.Handle(),
							createdContainer.Handle(),
						}))
					})
				})

				Context("when the same worker base resource type is saved", func() {
					BeforeEach(func() {
						sameWorker := defaultWorkerPayload

						defaultWorker, err = workerFactory.SaveWorker(sameWorker, 0)
						Expect(err).NotTo(HaveOccurred())
					})

					It("does not mark the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(HaveLen(0))
					})
				})
			})
		})

		Describe("get containers", func() {
			var (
				createdContainer dbng.CreatedContainer
				resourceCache    *dbng.UsedResourceCache
			)

			BeforeEach(func() {
				resourceCache, err = resourceCacheFactory.FindOrCreateResourceCache(
					logger,
					dbng.ForResource{defaultResource.ID},
					"some-base-resource-type",
					atc.Version{"some": "version"},
					atc.Source{"some": "source"},
					atc.Params{},
					defaultPipeline.ID(),
					atc.ResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err := defaultTeam.CreateResourceGetContainer(defaultWorker.Name(), resourceCache, "some-task")
				Expect(err).NotTo(HaveOccurred())

				createdContainer, err = creatingContainer.Created()
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

				It("marks the container for deletion", func() {
					err = containerFactory.MarkContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(deletingContainers).To(HaveLen(1))
					Expect(deletingContainers[0].Handle()).To(Equal(createdContainer.Handle()))
				})
			})
		})
	})
})
