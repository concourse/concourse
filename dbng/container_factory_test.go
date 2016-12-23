package dbng_test

import (
	"errors"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/atc/dbng"
	"github.com/lib/pq"
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
			destroyingContainer, err := containerFactory.ContainerDestroying(defaultCreatedContainer)
			Expect(err).NotTo(HaveOccurred())

			deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
			Expect(err).NotTo(HaveOccurred())

			Expect(deletingContainers).To(HaveLen(1))

			destroyedContainer := deletingContainers[0]
			Expect(destroyedContainer.ID).To(Equal(destroyingContainer.ID))
			Expect(destroyedContainer.Handle).To(Equal(destroyingContainer.Handle))
			Expect(destroyedContainer.WorkerName).To(Equal(destroyingContainer.WorkerName))
		})

	})

	Describe("MarkBuildContainersForDeletion", func() {
		var (
			listener *pq.Listener

			dbBuild db.Build
			build   *dbng.Build

			creatingContainer *dbng.CreatingContainer

			pipelineDB db.PipelineDB
		)

		BeforeEach(func() {
			oldDBConn := db.Wrap(sqlDB)

			listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
			Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
			bus := db.NewNotificationsBus(listener, oldDBConn)

			pgxConn := postgresRunner.OpenPgx()
			fakeConnector := new(lockfakes.FakeConnector)
			retryableConn := &lock.RetryableConn{Connector: fakeConnector, Conn: pgxConn}

			lockFactory := lock.NewLockFactory(retryableConn)
			teamDBFactory := db.NewTeamDBFactory(oldDBConn, bus, lockFactory)

			teamDB := teamDBFactory.GetTeamDB("default-team")

			savedPipeline, ok, err := teamDB.SaveConfig("some-pipeline", atc.Config{
				Jobs: []atc.JobConfig{
					{
						Name:   "some-job",
						Public: true,
						Plan: atc.PlanSequence{
							{
								Task:           "some-task",
								Privileged:     true,
								TaskConfigPath: "some/config/path.yml",
								TaskConfig: &atc.TaskConfig{
									Image: "some-image",
								},
							},
						},
					},
				},
			}, db.ConfigVersion(0), db.PipelineUnpaused)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			pipelineDBFactory := db.NewPipelineDBFactory(oldDBConn, bus, lockFactory)
			pipelineDB = pipelineDBFactory.Build(savedPipeline)

			dbBuild, err = pipelineDB.CreateJobBuild("some-job")
			Expect(err).NotTo(HaveOccurred())

			build = &dbng.Build{
				ID: dbBuild.ID(),
			}

			creatingContainer, err = containerFactory.CreateBuildContainer(defaultWorker, build, atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the container is creating", func() {
			It("does not mark the container for deletion", func() {
				err = containerFactory.MarkBuildContainersForDeletion()
				Expect(err).NotTo(HaveOccurred())

				deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
				Expect(err).NotTo(HaveOccurred())

				Expect(deletingContainers).To(BeEmpty())
			})
		})

		Context("when the container is created", func() {
			var (
				createdContainer *dbng.CreatedContainer

				ok bool
			)

			BeforeEach(func() {
				createdContainer, err = containerFactory.ContainerCreated(creatingContainer)
			})

			Context("when the build is not finished", func() {
				It("does not mark the container for deletion", func() {
					err = containerFactory.MarkBuildContainersForDeletion()
					Expect(err).NotTo(HaveOccurred())

					deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
					Expect(err).NotTo(HaveOccurred())

					Expect(deletingContainers).To(BeEmpty())

				})
			})

			Context("when the build is finished", func() {
				BeforeEach(func() {
					ok, err = dbBuild.Start("engine", "metadata")
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(BeTrue())
				})

				Context("and there is a more recent build which has finished", func() {
					var (
						laterBuild   *dbng.Build
						dbLaterBuild db.Build

						laterCreatingContainer *dbng.CreatingContainer
					)

					BeforeEach(func() {
						err = dbBuild.MarkAsFailed(errors.New("some-error"))
						Expect(err).NotTo(HaveOccurred())

						dbLaterBuild, err = pipelineDB.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						laterBuild = &dbng.Build{
							ID: dbLaterBuild.ID(),
						}

						ok, err = dbLaterBuild.Start("engine", "metadata")
						Expect(err).NotTo(HaveOccurred())
						Expect(ok).To(BeTrue())

						err = dbLaterBuild.Finish(db.StatusSucceeded)
						Expect(err).NotTo(HaveOccurred())

						laterCreatingContainer, err = containerFactory.CreateBuildContainer(defaultWorker, laterBuild, atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
						Expect(err).NotTo(HaveOccurred())

						_, err = containerFactory.ContainerCreated(laterCreatingContainer)
						Expect(err).NotTo(HaveOccurred())
					})

					It("marks the older container for deletion", func() {
						err = containerFactory.MarkBuildContainersForDeletion()
						Expect(err).NotTo(HaveOccurred())

						deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
						Expect(err).NotTo(HaveOccurred())

						Expect(deletingContainers).ToNot(BeEmpty())
						Expect(deletingContainers[0].ID).To(Equal(build.ID))
					})
				})

				Context("and there is a more recent build which is started and not finished", func() {
					var (
						laterBuild   *dbng.Build
						dbLaterBuild db.Build

						laterCreatingContainer *dbng.CreatingContainer
					)

					BeforeEach(func() {
						dbLaterBuild, err = pipelineDB.CreateJobBuild("some-job")
						Expect(err).NotTo(HaveOccurred())

						laterBuild = &dbng.Build{
							ID: dbLaterBuild.ID(),
						}

						ok, err = dbLaterBuild.Start("engine", "metadata")
						Expect(err).NotTo(HaveOccurred())
						Expect(ok).To(BeTrue())

						laterCreatingContainer, err = containerFactory.CreateBuildContainer(defaultWorker, laterBuild, atc.PlanID("some-job"), dbng.ContainerMetadata{Type: "task", Name: "some-task"})
						Expect(err).NotTo(HaveOccurred())

						_, err = containerFactory.ContainerCreated(laterCreatingContainer)
						Expect(err).NotTo(HaveOccurred())
					})

					Context("when the build is failing", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusFailed)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build errors", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusErrored)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build is aborted", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusAborted)
							Expect(err).NotTo(HaveOccurred())

						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build passed", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("marks the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).ToNot(BeEmpty())
							Expect(deletingContainers[0].ID).To(Equal(build.ID))
						})
					})
				})

				Context("when this is the most recent build", func() {
					Context("when the build is failing", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusFailed)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build errors", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusErrored)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build is aborted", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusAborted)
							Expect(err).NotTo(HaveOccurred())
						})

						It("does not mark the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).To(BeEmpty())
						})
					})

					Context("when the build passed", func() {
						BeforeEach(func() {
							err = dbBuild.Finish(db.StatusSucceeded)
							Expect(err).NotTo(HaveOccurred())
						})

						It("marks the container for deletion", func() {
							err = containerFactory.MarkBuildContainersForDeletion()
							Expect(err).NotTo(HaveOccurred())

							deletingContainers, err := containerFactory.FindContainersMarkedForDeletion()
							Expect(err).NotTo(HaveOccurred())

							Expect(deletingContainers).ToNot(BeEmpty())
							Expect(deletingContainers[0].ID).To(Equal(build.ID))
						})
					})
				})
			})
		})

	})
})
