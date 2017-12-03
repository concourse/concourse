package db_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"

	. "github.com/onsi/ginkgo"
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
			ActiveContainers: 140,
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
			StartTime: 55,
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

			It("replaces outdated worker resource type", func() {
				beforeIDs := resourceTypeIDs("some-name")
				Expect(len(beforeIDs)).To(Equal(2))

				atcWorker.ResourceTypes[0].Version = "some-new-version"

				_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				afterIDs := resourceTypeIDs("some-name")
				Expect(len(afterIDs)).To(Equal(2))

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
				Expect(foundWorker.ActiveContainers()).To(Equal(140))
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
				Expect(foundWorker.StartTime()).To(Equal(int64(55)))
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

				It("sets its garden and baggageclaim address to nil", func() {
					foundWorker, found, err := workerFactory.GetWorker("some-name")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeTrue())
					Expect(foundWorker.GardenAddr()).To(BeNil())
					Expect(foundWorker.BaggageclaimURL()).To(BeNil())
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
		)

		BeforeEach(func() {
			ttl = 5 * time.Minute
			epsilon = 30 * time.Second
			activeContainers = 0

			atcWorker.ActiveContainers = activeContainers
		})

		Context("when the worker is present", func() {
			JustBeforeEach(func() {
				_, err := workerFactory.SaveWorker(atcWorker, 1*time.Second)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates the expires field and the number of active containers", func() {
				atcWorker.ActiveContainers = 1

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

					Expect(stalledWorker.GardenAddr()).To(BeNil())
					Expect(stalledWorker.BaggageclaimURL()).To(BeNil())

					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(*foundWorker.GardenAddr()).To(Equal("some-garden-addr"))
					Expect(*foundWorker.BaggageclaimURL()).To(Equal("some-bc-url"))
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
})
