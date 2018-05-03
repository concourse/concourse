package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/creds"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ContainerRepository", func() {
	Describe("FindOrphanedContainers", func() {
		Describe("check containers", func() {
			var (
				creatingContainer          db.CreatingContainer
				resourceConfigCheckSession db.ResourceConfigCheckSession
			)

			expiries := db.ContainerOwnerExpiries{
				GraceTime: 2 * time.Minute,
				Min:       5 * time.Minute,
				Max:       1 * time.Hour,
			}

			BeforeEach(func() {
				var err error
				resourceConfigCheckSession, err = resourceConfigCheckSessionFactory.FindOrCreateResourceConfigCheckSession(
					logger,
					"some-base-resource-type",
					atc.Source{"some": "source"},
					creds.VersionedResourceTypes{},
					expiries,
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewResourceConfigCheckSessionContainerOwner(resourceConfigCheckSession, defaultTeam.ID()), fullMetadata)
				Expect(err).NotTo(HaveOccurred())
			})

			JustBeforeEach(func() {
				err := resourceConfigCheckSessionLifecycle.CleanExpiredResourceConfigCheckSessions()
				Expect(err).NotTo(HaveOccurred())
			})

			Context("when check container best if use by date is expired", func() {
				BeforeEach(func() {
					_, err := psql.Update("resource_config_check_sessions").
						Set("expires_at", sq.Expr("NOW() - '1 second'::INTERVAL")).
						Where(sq.Eq{"id": resourceConfigCheckSession.ID()}).
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
					_, err := psql.Update("resource_config_check_sessions").
						Set("expires_at", sq.Expr("NOW() + '1 hour'::INTERVAL")).
						Where(sq.Eq{"id": resourceConfigCheckSession.ID()}).
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

				creatingContainer, err = defaultTeam.CreateContainer(
					defaultWorker.Name(),
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan"),
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

				creatingTaskContainer, err = defaultTeam.CreateContainer(
					defaultWorker.Name(),
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan"),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewImageCheckContainerOwner(creatingTaskContainer), fullMetadata)
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

				creatingTaskContainer, err = defaultTeam.CreateContainer(
					defaultWorker.Name(),
					db.NewBuildStepContainerOwner(build.ID(), "simple-plan"),
					fullMetadata,
				)
				Expect(err).NotTo(HaveOccurred())

				creatingContainer, err = defaultTeam.CreateContainer(defaultWorker.Name(), db.NewImageGetContainerOwner(creatingTaskContainer), fullMetadata)
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

	Describe("FindFailedContainers", func() {
		var failedErr error
		var failedContainers []db.FailedContainer

		JustBeforeEach(func() {
			failedContainers, failedErr = containerRepository.FindFailedContainers()
		})

		ItClosesConnection := func() {
			It("closes the connection", func() {
				closed := make(chan bool)

				go func() {
					_, _ = containerRepository.FindFailedContainers()
					closed <- true
				}()

				Eventually(closed).Should(Receive())
			})
		}

		Context("when there are failed containers", func() {
			BeforeEach(func() {

				result, err := psql.Insert("containers").SetMap(map[string]interface{}{
					"state":        "failed",
					"handle":       "123-456-abc-def",
					"worker_name":  defaultWorker.Name(),
					"hijacked":     false,
					"discontinued": false,
				}).RunWith(dbConn).Exec()

				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))
			})

			It("returns all failed containers", func() {
				Expect(failedContainers).To(HaveLen(1))
			})
			It("does not return an error", func() {
				Expect(failedErr).ToNot(HaveOccurred())
			})
			ItClosesConnection()
		})

		Context("when there are no failed containers", func() {
			It("returns an empty array", func() {
				Expect(failedContainers).To(HaveLen(0))
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
						"state":  "failed",
						"handle": "123-456-abc-def",
					}).RunWith(dbConn).Exec()

					Expect(err).ToNot(HaveOccurred())
					Expect(result.RowsAffected()).To(Equal(int64(1)))

				})
				It("returns an error", func() {
					Expect(failedErr).To(HaveOccurred())

				})
				ItClosesConnection()
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
					"state":        "destroying",
					"handle":       "123-456-abc-def",
					"worker_name":  defaultWorker.Name(),
					"hijacked":     false,
					"discontinued": false,
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

	Describe("RemoveDestroyingContainers", func() {
		var failedErr error
		var numDeleted int
		var handles []string

		JustBeforeEach(func() {
			numDeleted, failedErr = containerRepository.RemoveDestroyingContainers(defaultWorker.Name(), handles)
		})
		ItClosesConnection := func() {
			It("closes the connection", func() {
				closed := make(chan bool)

				go func() {
					_, _ = containerRepository.RemoveDestroyingContainers(defaultWorker.Name(), handles)
					closed <- true
				}()

				Eventually(closed).Should(Receive())
			})
		}

		Context("when there are containers to destroy", func() {

			Context("when container is in destroying state", func() {
				BeforeEach(func() {
					handles = []string{"some-handle1", "some-handle2"}
					result, err := psql.Insert("containers").SetMap(map[string]interface{}{
						"state":        "destroying",
						"handle":       "123-456-abc-def",
						"worker_name":  defaultWorker.Name(),
						"hijacked":     false,
						"discontinued": false,
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
						"state":        "destroying",
						"handle":       "123-456-abc-def",
						"worker_name":  defaultWorker.Name(),
						"hijacked":     false,
						"discontinued": false,
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
						"state":        "creating",
						"handle":       "123-456-abc-def",
						"worker_name":  defaultWorker.Name(),
						"hijacked":     false,
						"discontinued": false,
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

			ItClosesConnection()
		})

		Context("when there are no containers to destroy", func() {
			BeforeEach(func() {
				handles = []string{"some-handle1", "some-handle2"}

				result, err := psql.Insert("containers").SetMap(
					map[string]interface{}{
						"state":        "destroying",
						"handle":       "some-handle1",
						"worker_name":  defaultWorker.Name(),
						"hijacked":     false,
						"discontinued": false,
					},
				).RunWith(dbConn).Exec()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.RowsAffected()).To(Equal(int64(1)))

				result, err = psql.Insert("containers").SetMap(
					map[string]interface{}{
						"state":        "destroying",
						"handle":       "some-handle2",
						"worker_name":  defaultWorker.Name(),
						"hijacked":     false,
						"discontinued": false,
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
		})
	})
})
