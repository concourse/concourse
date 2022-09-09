package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/dbtest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerFactory", func() {
	var (
		atcWorker atc.Worker
		worker    db.Worker
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
			ActiveVolumes:    550,
			ResourceTypes: []atc.WorkerResourceType{
				{
					Type:       "some-resource-type",
					Image:      "some-image",
					Version:    "some-version",
					Privileged: true,
				},
				{
					Type:       "other-resource-type",
					Image:      "other-image",
					Version:    "other-version",
					Privileged: false,
				},
			},
			Platform:  "some-platform",
			Tags:      atc.Tags{"some", "tags"},
			Name:      "some-name",
			StartTime: 1565367209,
		}
	})

	Describe("SaveWorker", func() {
		resourceTypeIDs := func(workerName string) map[string]int {
			ids := map[string]int{}
			rows, err := psql.Select("w.id", "b.name").
				From("worker_base_resource_types w").
				Join("base_resource_types AS b ON w.base_resource_type_id = b.id").
				Where(sq.Eq{"w.worker_name": workerName}).
				RunWith(dbConn).
				Query()
			Expect(err).NotTo(HaveOccurred())
			for rows.Next() {
				var id int
				var name string
				err = rows.Scan(&id, &name)
				Expect(err).NotTo(HaveOccurred())
				ids[name] = id
			}
			return ids
		}

		Context("when the worker already exists", func() {
			BeforeEach(func() {
				var err error
				worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("saves resource types", func() {
				worker, found, err := workerFactory.GetWorker(atcWorker.Name)
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				Expect(worker.ResourceTypes()).To(Equal(atcWorker.ResourceTypes))
			})

			It("removes old worker resource type", func() {
				atcWorker.ResourceTypes = []atc.WorkerResourceType{
					{
						Type:       "other-resource-type",
						Image:      "other-image",
						Version:    "other-version",
						Privileged: false,
					},
				}

				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				var count int
				err = psql.Select("count(*)").
					From("worker_base_resource_types").
					Where(sq.Eq{"worker_name": "some-name"}).
					RunWith(dbConn).
					QueryRow().Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(1))
			})

			It("replaces outdated worker resource type image", func() {
				beforeIDs := resourceTypeIDs("some-name")
				Expect(len(beforeIDs)).To(Equal(2))

				atcWorker.ResourceTypes[0].Image = "some-wild-new-image"

				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				afterIDs := resourceTypeIDs("some-name")
				Expect(len(afterIDs)).To(Equal(2))

				Expect(afterIDs).ToNot(Equal(beforeIDs))

				Expect(beforeIDs["some-resource-type"]).ToNot(Equal(afterIDs["some-resource-type"]))
				Expect(beforeIDs["other-resource-type"]).To(Equal(afterIDs["other-resource-type"]))
			})

			It("replaces outdated worker resource type version", func() {
				beforeIDs := resourceTypeIDs("some-name")
				Expect(len(beforeIDs)).To(Equal(2))

				atcWorker.ResourceTypes[0].Version = "some-wild-new-version"

				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				afterIDs := resourceTypeIDs("some-name")
				Expect(len(afterIDs)).To(Equal(2))

				Expect(afterIDs).ToNot(Equal(beforeIDs))

				Expect(beforeIDs["some-resource-type"]).ToNot(Equal(afterIDs["some-resource-type"]))
				Expect(beforeIDs["other-resource-type"]).To(Equal(afterIDs["other-resource-type"]))
			})

			Context("when the worker is in stalled state", func() {
				BeforeEach(func() {
					_, err := workerFactory.SaveWorker(atcWorker, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					_, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("repopulates the garden address", func() {
					savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					Expect(savedWorker.Name()).To(Equal("some-name"))
					Expect(*savedWorker.GardenAddr()).To(Equal("some-garden-addr"))
					Expect(savedWorker.State()).To(Equal(db.WorkerStateRunning))
				})
			})

			Context("when the worker has a new version", func() {
				BeforeEach(func() {
					atcWorker.Version = "1.0.0"
				})

				It("updates the version of the worker", func() {
					savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					Expect(worker.Version()).To(BeNil())
					Expect(*savedWorker.Version()).To(Equal("1.0.0"))
				})
			})
		})

		Context("no worker with same name exists", func() {
			BeforeEach(func() {
				atcWorker.Version = "1.0.0"
			})

			It("saves worker", func() {
				savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedWorker.Name()).To(Equal("some-name"))
				Expect(*savedWorker.GardenAddr()).To(Equal("some-garden-addr"))
				Expect(savedWorker.State()).To(Equal(db.WorkerStateRunning))
				Expect(*savedWorker.Version()).To(Equal("1.0.0"))
			})

			It("saves worker resource types as base resource types", func() {
				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				var count int
				err = psql.Select("count(*)").
					From("worker_base_resource_types").
					Where(sq.Eq{"worker_name": "some-name"}).
					RunWith(dbConn).
					QueryRow().Scan(&count)
				Expect(err).NotTo(HaveOccurred())
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe("GetWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the worker", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundWorker.Name()).To(Equal("some-name"))
				Expect(*foundWorker.GardenAddr()).To(Equal("some-garden-addr"))
				Expect(foundWorker.State()).To(Equal(db.WorkerStateRunning))
				Expect(*foundWorker.BaggageclaimURL()).To(Equal("some-bc-url"))
				Expect(foundWorker.HTTPProxyURL()).To(Equal("some-http-proxy-url"))
				Expect(foundWorker.HTTPSProxyURL()).To(Equal("some-https-proxy-url"))
				Expect(foundWorker.NoProxy()).To(Equal("some-no-proxy"))
				Expect(foundWorker.Ephemeral()).To(Equal(true))
				Expect(foundWorker.ActiveContainers()).To(Equal(140))
				Expect(foundWorker.ActiveVolumes()).To(Equal(550))
				Expect(foundWorker.ResourceTypes()).To(Equal([]atc.WorkerResourceType{
					{
						Type:       "some-resource-type",
						Image:      "some-image",
						Version:    "some-version",
						Privileged: true,
					},
					{
						Type:    "other-resource-type",
						Image:   "other-image",
						Version: "other-version",
					},
				}))
				Expect(foundWorker.Platform()).To(Equal("some-platform"))
				Expect(foundWorker.Tags()).To(Equal([]string{"some", "tags"}))
				Expect(foundWorker.StartTime().Unix()).To(Equal(int64(1565367209)))
				Expect(foundWorker.State()).To(Equal(db.WorkerStateRunning))
			})

			Context("when worker is stalled", func() {
				BeforeEach(func() {
					_, err := workerFactory.SaveWorker(atcWorker, -1*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					stalled, err := workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
					Expect(stalled).To(ContainElement("some-name"))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns false but no error", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("VisibleWorkers", func() {
		BeforeEach(func() {
			postgresRunner.Truncate()
		})

		Context("when there are public and private workers on multiple teams", func() {
			BeforeEach(func() {
				team1, err := teamFactory.CreateTeam(atc.Team{Name: "some-team"})
				Expect(err).NotTo(HaveOccurred())
				team2, err := teamFactory.CreateTeam(atc.Team{Name: "some-other-team"})
				Expect(err).NotTo(HaveOccurred())
				team3, err := teamFactory.CreateTeam(atc.Team{Name: "not-this-team"})
				Expect(err).NotTo(HaveOccurred())

				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = team1.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-other-new-worker"
				atcWorker.GardenAddr = "some-other-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-other-bc-url"
				_, err = team2.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "not-this-worker"
				atcWorker.GardenAddr = "not-this-garden-addr"
				atcWorker.BaggageclaimURL = "not-this-bc-url"
				_, err = team3.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds visble workers for the given teams", func() {
				workers, err := workerFactory.VisibleWorkers([]string{"some-team", "some-other-team"})
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(3))

				w1, found, err := workerFactory.GetWorker("some-name")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				w2, found, err := workerFactory.GetWorker("some-new-worker")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				w3, found, err := workerFactory.GetWorker("some-other-new-worker")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				w4, found, err := workerFactory.GetWorker("not-this-worker")
				Expect(found).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())

				Expect(workers).To(ConsistOf(w1, w2, w3))
				Expect(workers).NotTo(ContainElement(w4))
			})
		})

		Context("when there are no workers", func() {
			It("returns an error", func() {
				workers, err := workerFactory.VisibleWorkers([]string{"some-team"})
				Expect(err).NotTo(HaveOccurred())
				Expect(workers).To(BeEmpty())
			})
		})
	})

	Describe("Workers", func() {
		BeforeEach(func() {
			postgresRunner.Truncate()
		})

		Context("when there are workers", func() {
			BeforeEach(func() {
				_, err := workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				atcWorker.BaggageclaimURL = "some-other-bc-url"
				_, err = workerFactory.SaveWorker(atcWorker, 0)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds them without error", func() {
				workers, err := workerFactory.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(2))

				strptr := func(s string) *string {
					return &s
				}

				Expect(workers).To(ConsistOf(
					And(
						WithTransform((db.Worker).Name, Equal("some-name")),
						WithTransform((db.Worker).GardenAddr, Equal(strptr("some-garden-addr"))),
						WithTransform((db.Worker).BaggageclaimURL, Equal(strptr("some-bc-url"))),
					),
					And(
						WithTransform((db.Worker).Name, Equal("some-new-worker")),
						WithTransform((db.Worker).GardenAddr, Equal(strptr("some-other-garden-addr"))),
						WithTransform((db.Worker).BaggageclaimURL, Equal(strptr("some-other-bc-url"))),
					),
				))
			})
		})

		Context("when there are no workers", func() {
			It("returns an error", func() {
				workers, err := workerFactory.Workers()
				Expect(err).NotTo(HaveOccurred())
				Expect(workers).To(BeEmpty())
			})
		})
	})

	Describe("HeartbeatWorker", func() {
		var (
			ttl              time.Duration
			epsilon          time.Duration
			activeContainers int
			activeVolumes    int
		)

		BeforeEach(func() {
			ttl = 5 * time.Minute
			epsilon = 30 * time.Second
			activeContainers = 0
			activeVolumes = 0

			atcWorker.ActiveContainers = activeContainers
			atcWorker.ActiveVolumes = activeVolumes
		})

		Context("when the worker is present", func() {
			JustBeforeEach(func() {
				_, err := workerFactory.SaveWorker(atcWorker, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the expires field, and the number of active containers and volumes", func() {
				atcWorker.ActiveContainers = 1
				atcWorker.ActiveVolumes = 3

				now := time.Now()
				By("current time")
				By(now.String())
				later := now.Add(ttl)
				By("later time")
				By(later.String())
				By("found worker expiry")
				foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
				Expect(err).NotTo(HaveOccurred())
				By(foundWorker.ExpiresAt().String())

				Expect(foundWorker.Name()).To(Equal(atcWorker.Name))
				Expect(foundWorker.ExpiresAt()).To(BeTemporally("~", later, epsilon))
				Expect(foundWorker.ActiveContainers()).To(And(Not(Equal(activeContainers)), Equal(1)))
				Expect(foundWorker.ActiveVolumes()).To(And(Not(Equal(activeVolumes)), Equal(3)))
				Expect(*foundWorker.GardenAddr()).To(Equal("some-garden-addr"))
				Expect(*foundWorker.BaggageclaimURL()).To(Equal("some-bc-url"))
			})

			Context("when the current state is landing", func() {
				BeforeEach(func() {
					atcWorker.State = string(db.WorkerStateLanding)
				})

				It("keeps the state as landing", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(db.WorkerStateLanding))
				})
			})

			Context("when the current state is retiring", func() {
				BeforeEach(func() {
					atcWorker.State = string(db.WorkerStateRetiring)
				})

				It("keeps the state as retiring", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(db.WorkerStateRetiring))
				})
			})

			Context("when the current state is running", func() {
				BeforeEach(func() {
					atcWorker.State = string(db.WorkerStateRunning)
				})

				It("keeps the state as running", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(db.WorkerStateRunning))
				})
			})

			Context("when the current state is stalled", func() {
				var (
					unresponsiveWorker db.Worker
					err                error
				)

				JustBeforeEach(func() {
					unresponsiveWorker, err = workerFactory.SaveWorker(atcWorker, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					_, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("sets the state as running", func() {
					stalledWorker, found, err := workerFactory.GetWorker(unresponsiveWorker.Name())
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())

					Expect(stalledWorker.State()).To(Equal(db.WorkerStateStalled))

					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(db.WorkerStateRunning))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(db.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("FindWorkerForContainerByOwner", func() {
		var (
			containerMetadata db.ContainerMetadata
			build             db.Build
			fakeOwner         *dbfakes.FakeContainerOwner
			otherFakeOwner    *dbfakes.FakeContainerOwner
		)

		BeforeEach(func() {
			var err error
			containerMetadata = db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			}
			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			fakeOwner = new(dbfakes.FakeContainerOwner)
			fakeOwner.FindReturns(sq.Eq{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, true, nil)
			fakeOwner.CreateReturns(map[string]interface{}{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, nil)

			otherFakeOwner = new(dbfakes.FakeContainerOwner)
			otherFakeOwner.FindReturns(sq.Eq{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  2,
			}, true, nil)
			otherFakeOwner.CreateReturns(map[string]interface{}{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  2,
			}, nil)
		})

		Context("when there are check containers", func() {
			Context("when there are multiple of the same containers on the global, team and tagged worker", func() {
				var (
					scenario *dbtest.Scenario
				)

				BeforeEach(func() {
					scenario = dbtest.Setup(
						builder.WithPipeline(atc.Config{
							Resources: atc.ResourceConfigs{
								{
									Name: "some-resource",
									Type: "some-base-resource-type",
									Source: atc.Source{
										"some": "source",
									},
								},
							},
						}),
						builder.WithWorker(atc.Worker{
							ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
							GardenAddr:      "some-tagged-garden-addr",
							BaggageclaimURL: "some-tagged-bc-url",
							Name:            "some-tagged-name",
							Tags:            []string{"some-tag"},
						}),
						builder.WithWorker(atc.Worker{
							ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
							GardenAddr:      "some-team-garden-addr",
							BaggageclaimURL: "some-team-bc-url",
							Name:            "some-team-name",
							Team:            "default-team",
						}),
						builder.WithWorker(atc.Worker{
							ResourceTypes:   []atc.WorkerResourceType{defaultWorkerResourceType},
							GardenAddr:      "some-other-garden-addr",
							BaggageclaimURL: "some-other-bc-url",
							Name:            "some-other-name",
						}),
						builder.WithResourceVersions(
							"some-resource",
						),
						builder.WithCheckContainer(
							"some-resource",
							"some-other-name",
						),
						builder.WithCheckContainer(
							"some-resource",
							"some-tagged-name",
						),
						builder.WithCheckContainer(
							"some-resource",
							"some-team-name",
						),
					)
				})

				It("should find all the workers that have the same container", func() {
					resource, found, err := scenario.Pipeline.Resource("some-resource")
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue(), "resource '%s' not found", "some-resource")

					rc, found, err := resourceConfigFactory.FindResourceConfigByID(resource.ResourceConfigID())
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue(), "resource config '%s' not found", rc.ID())

					owner := db.NewResourceConfigCheckSessionContainerOwner(
						rc.ID(),
						rc.OriginBaseResourceType().ID,
						db.ContainerOwnerExpiries{
							Min: 5 * time.Minute,
							Max: 5 * time.Minute,
						},
					)

					workers, err := workerFactory.FindWorkersForContainerByOwner(owner)
					Expect(err).ToNot(HaveOccurred())

					var workerNames []string
					for _, w := range workers {
						workerNames = append(workerNames, w.Name())
					}

					Expect(workerNames).To(ConsistOf([]string{"some-other-name", "some-tagged-name", "some-team-name"}))
				})
			})
		})

		Context("when there are build containers", func() {
			Context("when there is a creating container", func() {
				BeforeEach(func() {
					_, err := defaultWorker.CreateContainer(fakeOwner, containerMetadata)
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					workers, err := workerFactory.FindWorkersForContainerByOwner(fakeOwner)
					Expect(err).ToNot(HaveOccurred())
					Expect(workers).To(HaveLen(1))
					Expect(workers[0].Name()).To(Equal(defaultWorker.Name()))
				})

				It("does not find container for another team", func() {
					workers, err := workerFactory.FindWorkersForContainerByOwner(otherFakeOwner)
					Expect(err).ToNot(HaveOccurred())
					Expect(workers).To(HaveLen(0))
				})
			})

			Context("when there is a created container", func() {
				BeforeEach(func() {
					creatingContainer, err := defaultWorker.CreateContainer(fakeOwner, containerMetadata)
					Expect(err).ToNot(HaveOccurred())

					_, err = creatingContainer.Created()
					Expect(err).ToNot(HaveOccurred())
				})

				It("returns it", func() {
					workers, err := workerFactory.FindWorkersForContainerByOwner(fakeOwner)
					Expect(err).ToNot(HaveOccurred())
					Expect(workers).To(HaveLen(1))
					Expect(workers[0].Name()).To(Equal(defaultWorker.Name()))
				})

				It("does not find container for another team", func() {
					workers, err := workerFactory.FindWorkersForContainerByOwner(otherFakeOwner)
					Expect(err).ToNot(HaveOccurred())
					Expect(workers).To(HaveLen(0))
				})
			})

			Context("when there is no container", func() {
				It("returns nil", func() {
					bogusOwner := new(dbfakes.FakeContainerOwner)
					bogusOwner.FindReturns(sq.Eq{
						"build_id": build.ID() + 1,
						"plan_id":  "how-could-this-happen-to-me",
						"team_id":  1,
					}, true, nil)
					bogusOwner.CreateReturns(map[string]interface{}{
						"build_id": build.ID() + 1,
						"plan_id":  "how-could-this-happen-to-me",
						"team_id":  1,
					}, nil)

					workers, err := workerFactory.FindWorkersForContainerByOwner(bogusOwner)
					Expect(err).ToNot(HaveOccurred())
					Expect(workers).To(HaveLen(0))
				})
			})
		})
	})

	Describe("BuildContainersCountPerWorker", func() {
		var (
			fakeOwner      *dbfakes.FakeContainerOwner
			otherFakeOwner *dbfakes.FakeContainerOwner
			build          db.Build
		)

		BeforeEach(func() {
			var err error

			build, err = defaultTeam.CreateOneOffBuild()
			Expect(err).ToNot(HaveOccurred())

			worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).ToNot(HaveOccurred())

			fakeOwner = new(dbfakes.FakeContainerOwner)
			fakeOwner.FindReturns(sq.Eq{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, true, nil)
			fakeOwner.CreateReturns(map[string]interface{}{
				"build_id": build.ID(),
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, nil)

			otherFakeOwner = new(dbfakes.FakeContainerOwner)
			otherFakeOwner.FindReturns(sq.Eq{
				"build_id": nil,
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, true, nil)
			otherFakeOwner.CreateReturns(map[string]interface{}{
				"build_id": nil,
				"plan_id":  "simple-plan",
				"team_id":  1,
			}, nil)

			creatingContainer, err := defaultWorker.CreateContainer(fakeOwner, db.ContainerMetadata{
				Type:     "task",
				StepName: "some-task",
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = creatingContainer.Created()
			Expect(err).ToNot(HaveOccurred())

			_, err = defaultWorker.CreateContainer(otherFakeOwner, db.ContainerMetadata{
				Type: "check",
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = worker.CreateContainer(fakeOwner, db.ContainerMetadata{
				Type:     "task",
				StepName: "other-task",
			})
			Expect(err).ToNot(HaveOccurred())

			_, err = worker.CreateContainer(otherFakeOwner, db.ContainerMetadata{
				Type: "check",
			})
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns a map of worker to number of active build containers", func() {
			containersCountByWorker, err := workerFactory.BuildContainersCountPerWorker()
			Expect(err).ToNot(HaveOccurred())

			Expect(containersCountByWorker).To(HaveLen(2))
			Expect(containersCountByWorker[defaultWorker.Name()]).To(Equal(1))
			Expect(containersCountByWorker[worker.Name()]).To(Equal(1))
		})
	})
})
