package db_test

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	. "github.com/concourse/concourse/atc/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/types"
)

var _ = Describe("Worker", func() {
	var (
		atcWorker atc.Worker
		worker    Worker
	)

	BeforeEach(func() {
		atcWorker = atc.Worker{
			GardenAddr:       "some-garden-addr",
			BaggageclaimURL:  "some-bc-url",
			HTTPProxyURL:     "some-http-proxy-url",
			HTTPSProxyURL:    "some-https-proxy-url",
			NoProxy:          "some-no-proxy",
			Ephemeral:        true,
			ActiveContainers: 140,
			ResourceTypes: []atc.WorkerResourceType{
				{
					Type:    "some-resource-type",
					Image:   "some-image",
					Version: "some-version",
				},
				{
					Type:    "other-resource-type",
					Image:   "other-image",
					Version: "other-version",
				},
			},
			Platform:  "some-platform",
			Tags:      atc.Tags{"some", "tags"},
			Name:      "some-name",
			StartTime: 55912945,
		}
	})

	Describe("Land", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the worker is present", func() {
			It("marks the worker as `landing`", func() {
				err := worker.Land()
				Expect(err).NotTo(HaveOccurred())

				_, err = worker.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(worker.Name()).To(Equal(atcWorker.Name))
				Expect(worker.State()).To(Equal(WorkerStateLanding))
			})

			Context("when worker is already landed", func() {
				BeforeEach(func() {
					err := worker.Land()
					Expect(err).NotTo(HaveOccurred())
					_, err = workerLifecycle.LandFinishedLandingWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("keeps worker state as landed", func() {
					err := worker.Land()
					Expect(err).NotTo(HaveOccurred())
					_, err = worker.Reload()
					Expect(err).NotTo(HaveOccurred())

					Expect(worker.Name()).To(Equal(atcWorker.Name))
					Expect(worker.State()).To(Equal(WorkerStateLanded))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				err := worker.Delete()
				Expect(err).NotTo(HaveOccurred())

				err = worker.Land()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

	Describe("Retire", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the worker is present", func() {
			It("marks the worker as `retiring`", func() {
				err := worker.Retire()
				Expect(err).NotTo(HaveOccurred())

				_, err = worker.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(worker.Name()).To(Equal(atcWorker.Name))
				Expect(worker.State()).To(Equal(WorkerStateRetiring))
			})
		})

		Context("when the worker is not present", func() {
			BeforeEach(func() {
				err := worker.Delete()
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an error", func() {
				err := worker.Retire()
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		It("deletes the record for the worker", func() {
			err := worker.Delete()
			Expect(err).NotTo(HaveOccurred())

			_, found, err := workerFactory.GetWorker(atcWorker.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})

	Describe("Prune", func() {
		Context("when worker exists", func() {
			DescribeTable("worker in state",
				func(workerState string, errMatch GomegaMatcher) {
					worker, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      workerState,
					}, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					err = worker.Prune()
					Expect(err).To(errMatch)
				},

				Entry("running", "running", Equal(ErrCannotPruneRunningWorker)),
				Entry("landing", "landing", BeNil()),
				Entry("retiring", "retiring", BeNil()),
			)

			Context("when worker is stalled", func() {
				var pruneErr error
				BeforeEach(func() {
					worker, err := workerFactory.SaveWorker(atc.Worker{
						Name:       "worker-to-prune",
						GardenAddr: "1.2.3.4",
						State:      "running",
					}, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					_, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
					pruneErr = worker.Prune()
				})

				It("does not return error", func() {
					Expect(pruneErr).NotTo(HaveOccurred())
				})
			})
		})

		Context("when worker does not exist", func() {
			BeforeEach(func() {
				var err error
				worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				err = worker.Delete()
				Expect(err).NotTo(HaveOccurred())
			})

			It("raises ErrWorkerNotPresent", func() {
				err := worker.Prune()
				Expect(err).To(Equal(ErrWorkerNotPresent))
			})
		})
	})

	Describe("FindContainer/CreateContainer", func() {
		var (
			containerMetadata ContainerMetadata
			containerOwner    ContainerOwner

			foundCreatingContainer CreatingContainer
			foundCreatedContainer  CreatedContainer
			worker                 Worker
		)

		expiries := ContainerOwnerExpiries{
			Min: 5 * time.Minute,
			Max: 1 * time.Hour,
		}

		BeforeEach(func() {
			containerMetadata = ContainerMetadata{
				Type: "check",
			}

			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			atcWorker2 := atcWorker
			atcWorker2.Name = "some-name2"
			atcWorker2.GardenAddr = "some-garden-addr-other"
			otherWorker, err = workerFactory.SaveWorker(atcWorker2, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			resourceConfig, err := resourceConfigFactory.FindOrCreateResourceConfig(
				"some-resource-type",
				atc.Source{"some": "source"},
				atc.VersionedResourceTypes{},
			)
			Expect(err).ToNot(HaveOccurred())

			containerOwner = NewResourceConfigCheckSessionContainerOwner(
				resourceConfig.ID(),
				resourceConfig.OriginBaseResourceType().ID,
				expiries,
			)
		})

		JustBeforeEach(func() {
			var err error
			foundCreatingContainer, foundCreatedContainer, err = worker.FindContainer(containerOwner)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when there is a creating container", func() {
			var creatingContainer CreatingContainer

			BeforeEach(func() {
				var err error
				creatingContainer, err = worker.CreateContainer(containerOwner, containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("returns it", func() {
				Expect(foundCreatedContainer).To(BeNil())
				Expect(foundCreatingContainer).ToNot(BeNil())
			})

			Context("when finding on another worker", func() {
				BeforeEach(func() {
					worker = otherWorker
				})

				It("does not find it", func() {
					Expect(foundCreatingContainer).To(BeNil())
					Expect(foundCreatedContainer).To(BeNil())
				})
			})

			Context("when there is a created container", func() {
				BeforeEach(func() {
					_, err := creatingContainer.Created()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					Expect(foundCreatedContainer).ToNot(BeNil())
					Expect(foundCreatingContainer).To(BeNil())
				})

				Context("when finding on another worker", func() {
					BeforeEach(func() {
						worker = otherWorker
					})

					It("does not find it", func() {
						Expect(foundCreatingContainer).To(BeNil())
						Expect(foundCreatedContainer).To(BeNil())
					})
				})
			})

			Context("when the creating container is failed and gced", func() {
				BeforeEach(func() {
					var err error
					_, err = creatingContainer.Failed()
					Expect(err).ToNot(HaveOccurred())

					containerRepository := NewContainerRepository(dbConn)
					containersDestroyed, err := containerRepository.DestroyFailedContainers()
					Expect(containersDestroyed).To(Equal(1))
					Expect(err).ToNot(HaveOccurred())

					var checkSessions int
					err = dbConn.QueryRow("SELECT COUNT(*) FROM resource_config_check_sessions").Scan(&checkSessions)
					Expect(err).ToNot(HaveOccurred())
					Expect(checkSessions).To(Equal(1))
				})

				Context("and we create a new container", func() {
					BeforeEach(func() {
						_, err := worker.CreateContainer(containerOwner, containerMetadata)
						Expect(err).ToNot(HaveOccurred())
					})

					It("does not duplicate the resource config check session", func() {
						var checkSessions int
						err := dbConn.QueryRow("SELECT COUNT(*) FROM resource_config_check_sessions").Scan(&checkSessions)
						Expect(err).ToNot(HaveOccurred())
						Expect(checkSessions).To(Equal(1))
					})
				})
			})
		})

		Context("when there is no container", func() {
			It("returns nil", func() {
				Expect(foundCreatedContainer).To(BeNil())
				Expect(foundCreatingContainer).To(BeNil())
			})
		})

		Context("when the container has a meta type", func() {
			var container CreatingContainer

			Context("when the meta type is check", func() {
				BeforeEach(func() {
					containerMetadata = ContainerMetadata{
						Type: "check",
					}

					var err error
					container, err = worker.CreateContainer(containerOwner, containerMetadata)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns a container with empty team id", func() {
					var teamID sql.NullString

					err := dbConn.QueryRow(fmt.Sprintf("SELECT team_id FROM containers WHERE id='%d'", container.ID())).Scan(&teamID)
					Expect(err).ToNot(HaveOccurred())
					Expect(teamID.Valid).To(BeFalse())
				})
			})

			Context("when the meta type is not check", func() {
				BeforeEach(func() {
					containerMetadata = ContainerMetadata{
						Type: "get",
					}

					oneOffBuild, err := defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					container, err = worker.CreateContainer(NewBuildStepContainerOwner(oneOffBuild.ID(), atc.PlanID("1"), 1), containerMetadata)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns a container with a team id", func() {
					var teamID sql.NullString

					err := dbConn.QueryRow(fmt.Sprintf("SELECT team_id FROM containers WHERE id='%d'", container.ID())).Scan(&teamID)
					Expect(err).ToNot(HaveOccurred())
					Expect(teamID.Valid).To(BeTrue())
				})
			})
		})

		Context("when the container has limits", func() {
			var container CreatingContainer
			BeforeEach(func() {
				memoryLimit := atc.MemoryLimit(1024)
				containerMetadata = ContainerMetadata{
					Type:        "check",
					MemoryLimit: &memoryLimit,
				}

				var err error
				container, err = worker.CreateContainer(containerOwner, containerMetadata)
				Expect(err).ToNot(HaveOccurred())
			})

			It("persists the configured limits", func() {
				var memory sql.NullInt64

				err := dbConn.QueryRow(fmt.Sprintf("SELECT meta_memory_limit FROM containers WHERE id='%d'", container.ID())).Scan(&memory)
				Expect(err).ToNot(HaveOccurred())
				Expect(memory.Valid).To(BeTrue())
				Expect(memory.Int64).To(Equal(int64(1024)))
			})
		})
	})

	Describe("Active tasks", func() {
		BeforeEach(func() {
			var err error
			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the worker registers", func() {
			It("has no active tasks", func() {
				at, err := worker.ActiveTasks()
				Expect(err).ToNot(HaveOccurred())
				Expect(at).To(Equal(0))
			})
		})

		Context("when the active task is increased", func() {
			BeforeEach(func() {
				at, err := worker.IncreaseActiveTasks()
				Expect(err).ToNot(HaveOccurred())
				Expect(at).To(Equal(1))
			})

			It("increase the active tasks counter", func() {
				at, err := worker.ActiveTasks()
				Expect(err).ToNot(HaveOccurred())
				Expect(at).To(Equal(1))
			})

			Context("when the active task is decreased", func() {
				BeforeEach(func() {
					at, err := worker.DecreaseActiveTasks()
					Expect(err).ToNot(HaveOccurred())
					Expect(at).To(Equal(0))
				})

				It("reset the active tasks to 0", func() {
					at, err := worker.ActiveTasks()
					Expect(err).ToNot(HaveOccurred())
					Expect(at).To(Equal(0))
				})
			})
		})

		Context("when the active task is decreased below 0", func() {
			It("raise an error", func() {
				at, err := worker.DecreaseActiveTasks()
				Expect(err).To(HaveOccurred())
				Expect(at).To(Equal(0))
			})
		})
	})
})
