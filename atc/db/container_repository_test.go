package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/lib/pq"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerRepository", func() {
	Describe("FindOrphanedContainers", func() {
		Describe("check containers", func() {
			var (
				creatingContainer db.CreatingContainer
				resourceConfig    db.ResourceConfig
			)

			expiries := db.ContainerOwnerExpiries{
				Min: 5 * time.Minute,
				Max: 1 * time.Hour,
			}

			BeforeEach(func() {
				var err error
				resourceConfig, err = resourceConfigFactory.FindOrCreateResourceConfig(
					"some-base-resource-type",
					atc.Source{"some": "source"},
					atc.VersionedResourceTypes{},
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultWorker.CreateContainer(
					db.NewResourceConfigCheckSessionContainerOwner(
						resourceConfig.ID(),
						resourceConfig.OriginBaseResourceType().ID,
						expiries,
					),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err := resourceConfigCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessions()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when check container best if use by date is expired", func() {
				BeforeEach(func() {
					var rccsID int
					err := psql.Select("id").From("resource_config_check_sessions").
						Where(sq.Eq{"resource_config_id": resourceConfig.ID()}).RunWith(dbConn).QueryRow().Scan(&rccsID)

					_, err = psql.Update("resource_config_check_sessions").
						Set("expires_at", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": rccsID}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				Context("when container is creating", func() {
					It("finds the container for deletion", func() {
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					var rccsID int
					err := psql.Select("id").From("resource_config_check_sessions").
						Where(sq.Eq{"resource_config_id": resourceConfig.ID()}).RunWith(dbConn).QueryRow().Scan(&rccsID)

					_, err = psql.Update("resource_config_check_sessions").
						Set("expires_at", sq.Expr("NOW() + '1 hour'::INTERVAL")).
						Where(sq.Eq{"id": rccsID}).
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})

			Context("when resource configs are cleaned up", func() {
				BeforeEach(func() {
					_, err := psql.Delete("resource_config_check_sessions").
						RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())

					err = resourceConfigFactory.CleanUnreferencedConfigs()
					Expect(err).NotTo(HaveOccurred())
				})

				It("finds the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())
					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(HaveLen(0))
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(BeEmpty())
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		Describe("containers owned by a build", func() {
			var (
				creatingContainer db.CreatingContainer
				build             db.Build
			)

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultWorker.CreateContainer(
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan", defaultTeam.ID()),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the build is interceptible", func() {
				BeforeEach(func() {
					err := build.SetInterceptible(true)
					Expect(err).NotTo(HaveOccurred())
				})

				It("does not find container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
						creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		Describe("containers for checking images for creating containers", func() {
			var (
				creatingTaskContainer db.CreatingContainer
				creatingContainer     db.CreatingContainer
				build                 db.Build
			)

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingTaskContainer, err = defaultWorker.CreateContainer(
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan", defaultTeam.ID()),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultWorker.CreateContainer(db.NewImageCheckContainerOwner(creatingTaskContainer, defaultTeam.ID()), fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the container they're for is still creating", func() {
				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})

		Describe("containers for fetching images for creating containers", func() {
			var (
				creatingTaskContainer db.CreatingContainer
				creatingContainer     db.CreatingContainer
				build                 db.Build
			)

			BeforeEach(func() {
				var err error
				build, err = defaultJob.CreateBuild()
				Expect(err).NotTo(HaveOccurred())

				creatingTaskContainer, err = defaultWorker.CreateContainer(
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan", defaultTeam.ID()),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultWorker.CreateContainer(db.NewImageGetContainerOwner(creatingTaskContainer, defaultTeam.ID()), fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when the container they're for is still creating", func() {
				It("does not find the container for deletion", func() {
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
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
					creatingContainers, createdContainers, destroyingContainers, err := containerRepository.FindOrphanedContainers()
					Expect(err).NotTo(HaveOccurred())

					Expect(creatingContainers).To(HaveLen(1))
					Expect(creatingContainers[0].Handle()).To(Equal(creatingContainer.Handle()))
					Expect(createdContainers).To(BeEmpty())
					Expect(destroyingContainers).To(BeEmpty())
				})
			})
		})
	})

	Describe("DestroyFailedContainers", func() {
		var failedErr error
		var failedContainersLen int

		JustBeforeEach(func() {
			failedContainersLen, failedErr = containerRepository.DestroyFailedContainers()
		})

		Context("when there are failed containers", func() {
			BeforeEach(func() {
				result, err := psql.Insert("containers").
					SetMap(map[string]interface{}{
						"state":       atc.ContainerStateFailed,
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("returns all failed containers", func() {
				Expect(failedContainersLen).To(Equal(1))
			})

			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})
		})

		Context("when there are no failed containers", func() {
			It("returns an empty array", func() {
				Expect(failedContainersLen).To(Equal(0))
			})
			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})
		})

		Describe("errors", func() {
			Context("when the query cannot be executed", func() {
				BeforeEach(func() {
					err := dbConn.Close()
					Expect(err).ToNot(HaveOccurred())
				})
				AfterEach(func() {
					dbConn = postgresRunner.OpenConn()
				})
				It("returns an error", func() {
					Expect(failedErr).To(HaveOccurred())
				})
			})
		})
	})

	Describe("FindDestroyingContainers", func() {
		var failedErr error
		var destroyingContainers []string

		JustBeforeEach(func() {
			destroyingContainers, failedErr = containerRepository.FindDestroyingContainers(defaultWorker.Name())
		})
		ItClosesConnection := func() {
			It("closes the connection", func() {
				closed := make(chan bool)

				go func() {
					_, _ = containerRepository.FindDestroyingContainers(defaultWorker.Name())
					closed <- true
				}()

				Eventually(closed).Should(Receive())
			})
		}

		Context("when there are destroying containers", func() {
			BeforeEach(func() {
				result, err := psql.Insert("containers").SetMap(map[string]interface{}{
					"state":       "destroying",
					"handle":      "123-456-abc-def",
					"worker_name": defaultWorker.Name(),
				}).RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("returns all destroying containers", func() {
				Expect(destroyingContainers).To(HaveLen(1))
				Expect(destroyingContainers[0]).To(Equal("123-456-abc-def"))
			})

			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})

			ItClosesConnection()
		})

		Describe("errors", func() {
			Context("when the query cannot be executed", func() {
				BeforeEach(func() {
					err := dbConn.Close()
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					dbConn = postgresRunner.OpenConn()
				})

				It("returns an error", func() {
					Expect(failedErr).To(HaveOccurred())
				})

				ItClosesConnection()
			})

			Context("when there is an error iterating through the rows", func() {
				BeforeEach(func() {
					By("adding a row without expected values")
					result, err := psql.Insert("containers").SetMap(map[string]interface{}{
						"state":  "destroying",
						"handle": "123-456-abc-def",
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))

				})

				It("returns empty list", func() {
					Expect(destroyingContainers).To(HaveLen(0))
				})

				ItClosesConnection()
			})
		})
	})

	Describe("RemoveMissingContainers", func() {
		var (
			today        time.Time
			gracePeriod  time.Duration
			rowsAffected int
			err          error
		)

		BeforeEach(func() {
			today = time.Now()

			_, err = psql.Insert("workers").SetMap(map[string]interface{}{
				"name":  "running-worker",
				"state": "running",
			}).RunWith(dbConn).Exec()
			Expect(err).NotTo(HaveOccurred())

			_, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"handle":      "created-handle-1",
				"state":       atc.ContainerStateCreated,
				"worker_name": "running-worker",
			}).RunWith(dbConn).Exec()
			Expect(err).NotTo(HaveOccurred())

			_, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"handle":        "created-handle-2",
				"state":         atc.ContainerStateCreated,
				"worker_name":   "running-worker",
				"missing_since": today.Add(-5 * time.Minute),
			}).RunWith(dbConn).Exec()
			Expect(err).NotTo(HaveOccurred())

			_, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"handle":        "failed-handle-3",
				"state":         atc.ContainerStateFailed,
				"worker_name":   "running-worker",
				"missing_since": today.Add(-5 * time.Minute),
			}).RunWith(dbConn).Exec()
			Expect(err).NotTo(HaveOccurred())

			_, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"handle":        "destroying-handle-4",
				"state":         atc.ContainerStateDestroying,
				"worker_name":   "running-worker",
				"missing_since": today.Add(-10 * time.Minute),
			}).RunWith(dbConn).Exec()
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			rowsAffected, err = containerRepository.RemoveMissingContainers(gracePeriod)
		})

		Context("when no created/failed containers have expired", func() {
			BeforeEach(func() {
				gracePeriod = 7 * time.Minute
			})

			It("affects no containers", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(rowsAffected).To(Equal(0))
			})
		})

		Context("when some created containers have expired", func() {
			BeforeEach(func() {
				gracePeriod = 3 * time.Minute
			})

			It("affects the right containers and deletes created-handle-2", func() {
				result, err := psql.Select("*").From("containers").
					RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(3)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "created-handle-1"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "created-handle-2"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(0)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "failed-handle-3"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "destroying-handle-4"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})
		})

		Context("when worker is in stalled state", func() {
			BeforeEach(func() {
				gracePeriod = 3 * time.Minute

				_, err = psql.Insert("workers").SetMap(map[string]interface{}{
					"name":  "stalled-worker",
					"state": "stalled",
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Insert("containers").SetMap(map[string]interface{}{
					"handle":        "stalled-handle-5",
					"state":         atc.ContainerStateCreated,
					"worker_name":   "stalled-worker",
					"missing_since": today.Add(-10 * time.Minute),
				}).RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Update("containers").
					Set("worker_name", "stalled-worker").
					Where(sq.Eq{"handle": "failed-handle-3"}).
					RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())

				_, err = psql.Update("containers").
					Set("missing_since", today.Add(-5*time.Minute)).
					Where(sq.Eq{"handle": "destroying-handle-4"}).
					RunWith(dbConn).Exec()
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes containers missing for more than grace period, on running (unstalled) workers", func() {
				Expect(err).ToNot(HaveOccurred())
				Expect(rowsAffected).To(Equal(1))
			})

			It("does not delete containers on stalled workers", func() {
				result, err := psql.Select("*").From("containers").
					RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(4)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "created-handle-1"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "created-handle-2"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(0)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "failed-handle-3"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "destroying-handle-4"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Select("*").From("containers").
					Where(sq.Eq{"handle": "stalled-handle-5"}).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

		})
	})

	Describe("RemoveDestroyingContainers", func() {
		var failedErr error
		var numDeleted int
		var handles []string

		JustBeforeEach(func() {
			numDeleted, failedErr = containerRepository.RemoveDestroyingContainers(defaultWorker.Name(), handles)
		})

		Context("when there are containers to destroy", func() {

			Context("when container is in destroying state", func() {
				BeforeEach(func() {
					handles = []string{"some-handle1", "some-handle2"}
					result, err := psql.Insert("containers").SetMap(map[string]interface{}{
						"state":       atc.ContainerStateDestroying,
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("should destroy", func() {
					result, err := psql.Select("*").From("containers").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(0)))
				})
				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(1))
				})
				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when handles are empty list", func() {
				BeforeEach(func() {
					handles = []string{}
					result, err := psql.Insert("containers").SetMap(map[string]interface{}{
						"state":       atc.ContainerStateDestroying,
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})

				It("should destroy", func() {
					result, err := psql.Select("*").From("containers").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(0)))
				})

				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(1))
				})

				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})

			Context("when container is in create/creating state", func() {
				BeforeEach(func() {
					handles = []string{"some-handle1", "some-handle2"}
					result, err := psql.Insert("containers").SetMap(map[string]interface{}{
						"state":       "creating",
						"handle":      "123-456-abc-def",
						"worker_name": defaultWorker.Name(),
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("should not destroy", func() {
					result, err := psql.Select("*").From("containers").
						Where(sq.Eq{"handle": "123-456-abc-def"}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})
				It("returns the correct number of rows removed", func() {
					Expect(numDeleted).To(Equal(0))
				})
				It("does not return an error", func() {
					Expect(failedErr).ToNot(HaveOccurred())
				})
			})
		})

		Context("when there are no containers to destroy", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}

				result, err := psql.Insert("containers").SetMap(
					map[string]interface{}{
						"state":       "destroying",
						"handle":      "some-handle1",
						"worker_name": defaultWorker.Name(),
					},
				).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Insert("containers").SetMap(
					map[string]interface{}{
						"state":       "destroying",
						"handle":      "some-handle2",
						"worker_name": defaultWorker.Name(),
					},
				).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("doesn't destroy containers that are in handles", func() {
				result, err := psql.Select("*").From("containers").
					Where(sq.Eq{"handle": handles}).RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(2)))
			})

			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})
			It("returns the correct number of rows removed", func() {
				Expect(numDeleted).To(Equal(0))
			})
		})

		Describe("errors", func() {
			Context("when the query cannot be executed", func() {
				BeforeEach(func() {
					err := dbConn.Close()
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					dbConn = postgresRunner.OpenConn()
				})

				It("returns an error", func() {
					Expect(failedErr).To(HaveOccurred())
				})
			})
		})
	})

	Describe("UpdateContainersMissingSince", func() {
		var (
			today        time.Time
			err          error
			handles      []string
			missingSince pq.NullTime
		)

		BeforeEach(func() {
			result, err := psql.Insert("containers").SetMap(map[string]interface{}{
				"state":       atc.ContainerStateDestroying,
				"handle":      "some-handle1",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			result, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"state":       atc.ContainerStateDestroying,
				"handle":      "some-handle2",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			today = time.Date(2018, 9, 24, 0, 0, 0, 0, time.UTC)

			result, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"state":         atc.ContainerStateCreated,
				"handle":        "some-handle3",
				"worker_name":   defaultWorker.Name(),
				"missing_since": today,
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))
		})

		JustBeforeEach(func() {
			err = containerRepository.UpdateContainersMissingSince(defaultWorker.Name(), handles)
		})

		Context("when the reported handles is a subset", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1"}
			})

			Context("having the containers in the creating state in the db", func() {
				BeforeEach(func() {
					result, err := psql.Update("containers").
						Where(sq.Eq{"handle": "some-handle3"}).
						SetMap(map[string]interface{}{
							"state":         atc.ContainerStateCreating,
							"missing_since": nil,
						}).RunWith(dbConn).Exec()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))
				})

				It("does not mark as missing", func() {
					err = psql.Select("missing_since").From("containers").
						Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().
						Scan(&missingSince)
					Expect(err).ToNot(HaveOccurred())
					Expect(missingSince.Valid).To(BeFalse())
				})
			})

			It("should mark containers not in the subset and not already marked as missing", func() {
				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeTrue())

				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeTrue())
				Expect(missingSince.Time.Unix()).To(Equal(today.Unix()))
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the reported handles is the full set", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}
			})

			It("should not update", func() {
				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the reported handles includes a container marked as missing", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2", "some-handle3"}
			})

			It("should mark the previously missing container as not missing", func() {
				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle1"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle2"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())

				err = psql.Select("missing_since").From("containers").
					Where(sq.Eq{"handle": "some-handle3"}).RunWith(dbConn).QueryRow().Scan(&missingSince)
				Expect(err).ToNot(HaveOccurred())
				Expect(missingSince.Valid).To(BeFalse())
			})

			It("does not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("DestroyUnknownContainers", func() {
		var (
			err                     error
			workerReportedHandles   []string
			numberUnknownContainers int
		)

		BeforeEach(func() {
			result, err := psql.Insert("containers").SetMap(map[string]interface{}{
				"state":       atc.ContainerStateDestroying,
				"handle":      "some-handle1",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))

			result, err = psql.Insert("containers").SetMap(map[string]interface{}{
				"state":       atc.ContainerStateCreated,
				"handle":      "some-handle2",
				"worker_name": defaultWorker.Name(),
			}).RunWith(dbConn).Exec()

			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(Equal(int64(1)))
		})

		JustBeforeEach(func() {
			numberUnknownContainers, err = containerRepository.DestroyUnknownContainers(defaultWorker.Name(), workerReportedHandles)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there are containers on the worker that are not in the db", func() {
			var destroyingContainerHandles []string
			BeforeEach(func() {
				workerReportedHandles = []string{"some-handle3", "some-handle4"}
				destroyingContainerHandles = append(workerReportedHandles, "some-handle1")
			})

			It("adds new destroying containers to the database", func() {
				result, err := psql.Select("handle").
					From("containers").
					Where(sq.Eq{"state": atc.ContainerStateDestroying}).
					RunWith(dbConn).Query()

				Expect(err).ToNot(HaveOccurred())

				var handle string
				for result.Next() {
					err = result.Scan(&handle)
					Expect(err).ToNot(HaveOccurred())
					Expect(handle).Should(BeElementOf(destroyingContainerHandles))
				}
				Expect(numberUnknownContainers).To(Equal(2))
			})

			It("does not affect containers in any other state", func() {
				rows, err := psql.Select("handle").
					From("containers").
					Where(sq.Eq{"state": atc.ContainerStateCreated}).
					RunWith(dbConn).Query()

				Expect(err).ToNot(HaveOccurred())

				var handle string
				var rowsAffected int
				for rows.Next() {
					err = rows.Scan(&handle)
					Expect(err).ToNot(HaveOccurred())
					Expect(handle).To(Equal("some-handle2"))
					rowsAffected++
				}

				Expect(rowsAffected).To(Equal(1))
			})
		})

		Context("when there are no unknown containers on the worker", func() {
			BeforeEach(func() {
				workerReportedHandles = []string{"some-handle1", "some-handle2"}
			})

			It("should not try to destroy anything", func() {
				Expect(numberUnknownContainers).To(Equal(0))

				rows, err := psql.Select("handle").
					From("containers").
					Where(sq.Eq{"state": atc.ContainerStateDestroying}).
					RunWith(dbConn).Query()

				Expect(err).ToNot(HaveOccurred())

				var handle string
				var rowsAffected int
				for rows.Next() {
					err = rows.Scan(&handle)
					Expect(err).ToNot(HaveOccurred())
					Expect(handle).To(Equal("some-handle1"))
					rowsAffected++
				}

				Expect(rowsAffected).To(Equal(1))
			})
		})
	})
})
