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
		Describe("build containers", func() {
			var (
				creatingContainer dbng.CreatingContainer
				build             dbng.Build
			)

			BeforeEach(func() {
				build, err = defaultPipeline.CreateJobBuild("some-job")
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker, build.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
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

				Context("when the build is not finished", func() {
					It("does not mark the container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).To(BeEmpty())
					})
				})

				Context("when the build failed and there is a more recent build which has finished", func() {
					var (
						laterBuild             dbng.Build
						laterCreatingContainer dbng.CreatingContainer
						laterCreatedContainer  dbng.CreatedContainer
					)

					BeforeEach(func() {
						laterBuild, err = defaultPipeline.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						err = laterBuild.Finish(dbng.BuildStatusSucceeded)
						Expect(err).NotTo(HaveOccurred())

						err = build.Finish(dbng.BuildStatusFailed)
						Expect(err).NotTo(HaveOccurred())

						laterCreatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker, build.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
						Expect(err).NotTo(HaveOccurred())

						laterCreatedContainer, err = laterCreatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
					})

					It("marks the older container for deletion", func() {
						err = containerFactory.MarkContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).ToNot(BeEmpty())
						Expect(deletingContainers[0].Handle()).NotTo(Equal(laterCreatingContainer.Handle()))
					})

					Context("when containers are hijacked", func() {
						BeforeEach(func() {
							err := createdContainer.MarkAsHijacked()
							Expect(err).NotTo(HaveOccurred())

							err = laterCreatedContainer.MarkAsHijacked()
							Expect(err).NotTo(HaveOccurred())
						})

						It("returns hijacked containers in FindHijackedContainersForDeletion", func() {
							foundContainers, err := containerFactory.FindHijackedContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(foundContainers).To(HaveLen(2))
							Expect([]string{
								foundContainers[0].Handle(),
								foundContainers[1].Handle(),
							}).To(ConsistOf([]string{
								createdContainer.Handle(),
								laterCreatedContainer.Handle(),
							}))
						})

						It("does not mark containers for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})
				})

				Context("when there is a more recent build which is started and not finished", func() {
					var (
						laterBuild dbng.Build

						laterCreatingContainer dbng.CreatingContainer
					)

					BeforeEach(func() {
						laterBuild, err = defaultPipeline.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						err = laterBuild.SaveStatus(dbng.BuildStatusStarted)
						Expect(err).NotTo(HaveOccurred())

						laterCreatingContainer, err = defaultTeam.CreateBuildContainer(defaultWorker, laterBuild.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
						Expect(err).NotTo(HaveOccurred())

						_, err = laterCreatingContainer.Created()
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when the build is failing", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusFailed)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build errors", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusErrored)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build is aborted", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusAborted)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build passed", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("marks the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(HaveLen(1))
						})
					})
				})

				Context("when this is the most recent build", func() {
					Context("when the build is failing", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusFailed)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build errors", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusErrored)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build is aborted", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusAborted)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build passed", func() {
						BeforeEach(func() {
							err := build.Finish(dbng.BuildStatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("marks the container for deletion", func() {
							_, foundCreatedContainer, err := defaultTeam.FindBuildContainer(defaultWorker, build.ID(), atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
							Expect(err).NotTo(HaveOccurred())
							Expect(foundCreatedContainer).NotTo(BeNil())

							err = containerFactory.MarkContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(HaveLen(1))
						})
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
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfigForResource(
					logger,
					defaultResource.ID,
					"some-base-resource-type",
					atc.Source{"some": "source"},
					defaultPipeline.ID(),
					atc.ResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateResourceCheckContainer(defaultWorker, resourceConfig)
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
						_, foundCreatedContainer, err := defaultTeam.FindResourceCheckContainer(defaultWorker, resourceConfig)
						Expect(err).NotTo(HaveOccurred())
						Expect(foundCreatedContainer).NotTo(BeNil())

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
			})
		})

		Describe("get containers", func() {
			var (
				createdContainer dbng.CreatedContainer
				resourceCache    *dbng.UsedResourceCache
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

				creatingContainer, err := defaultTeam.CreateResourceGetContainer(defaultWorker, resourceCache, "some-task")
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
