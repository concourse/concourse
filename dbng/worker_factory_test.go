package dbng_test

import (
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/dbng"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WorkerFactory", func() {
	var (
		atcWorker atc.Worker
		worker    dbng.Worker
	)

	BeforeEach(func() {
		atcWorker = atc.Worker{
			GardenAddr:                 "some-garden-addr",
			BaggageclaimURL:            "some-bc-url",
			HTTPProxyURL:               "some-http-proxy-url",
			HTTPSProxyURL:              "some-https-proxy-url",
			NoProxy:                    "some-no-proxy",
			CertificatesPath:           "some-certificate-path",
			CertificatesSymlinkedPaths: []string{"some-certificate-symlinked-path-1", "some-certificate-symlinked-path-2"},
			ActiveContainers:           140,
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
			StartTime: 55,
		}
	})

	Describe("SaveWorker", func() {
		Context("the worker already exists", func() {
			BeforeEach(func() {
				var err error
				worker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes old worker resource type", func() {
				atcWorker.ResourceTypes = []atc.WorkerResourceType{
					{
						Type:    "other-resource-type",
						Image:   "other-image",
						Version: "other-version",
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

			Context("the worker is in stalled state", func() {
				var stalled []string
				BeforeEach(func() {
					_, err := workerFactory.SaveWorker(atcWorker, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					stalled, err = workerLifecycle.StallUnresponsiveWorkers()
					Expect(err).NotTo(HaveOccurred())
				})

				It("repopulates the garden address", func() {
					savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
					Expect(err).NotTo(HaveOccurred())
					Expect(savedWorker.Name()).To(Equal("some-name"))
					Expect(*savedWorker.GardenAddr()).To(Equal("some-garden-addr"))
					Expect(savedWorker.State()).To(Equal(dbng.WorkerStateRunning))
				})
			})
		})

		Context("no worker with same name exists", func() {
			It("saves worker", func() {
				savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(savedWorker.Name()).To(Equal("some-name"))
				Expect(*savedWorker.GardenAddr()).To(Equal("some-garden-addr"))
				Expect(savedWorker.State()).To(Equal(dbng.WorkerStateRunning))
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
			var createdWorker dbng.Worker
			BeforeEach(func() {
				var err error
				createdWorker, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the worker", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundWorker.Name()).To(Equal("some-name"))
				Expect(*foundWorker.GardenAddr()).To(Equal("some-garden-addr"))
				Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRunning))
				Expect(*foundWorker.BaggageclaimURL()).To(Equal("some-bc-url"))
				Expect(foundWorker.HTTPProxyURL()).To(Equal("some-http-proxy-url"))
				Expect(foundWorker.HTTPSProxyURL()).To(Equal("some-https-proxy-url"))
				Expect(foundWorker.NoProxy()).To(Equal("some-no-proxy"))
				Expect(foundWorker.CertificatesPath()).To(Equal("some-certificate-path"))
				Expect(foundWorker.CertificatesSymlinkedPaths()).To(Equal([]string{"some-certificate-symlinked-path-1", "some-certificate-symlinked-path-2"}))
				Expect(foundWorker.ActiveContainers()).To(Equal(140))
				Expect(foundWorker.ResourceTypes()).To(Equal([]atc.WorkerResourceType{
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
				}))
				Expect(foundWorker.Platform()).To(Equal("some-platform"))
				Expect(foundWorker.Tags()).To(Equal([]string{"some", "tags"}))
				Expect(foundWorker.StartTime()).To(Equal(int64(55)))
				Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRunning))
			})

			Context("when worker is stalled", func() {
				BeforeEach(func() {
					var err error
					createdWorker, err = workerFactory.SaveWorker(atcWorker, -1*time.Minute)
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

				Expect(workers[0].Name()).To(Equal("some-name"))
				Expect(*workers[0].GardenAddr()).To(Equal("some-garden-addr"))
				Expect(*workers[0].BaggageclaimURL()).To(Equal("some-bc-url"))

				Expect(workers[1].Name()).To(Equal("some-new-worker"))
				Expect(*workers[1].GardenAddr()).To(Equal("some-other-garden-addr"))
				Expect(*workers[1].BaggageclaimURL()).To(Equal("some-other-bc-url"))
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
					atcWorker.State = string(dbng.WorkerStateLanding)
				})

				It("keeps the state as landing", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(dbng.WorkerStateLanding))
				})
			})

			Context("when the current state is retiring", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateRetiring)
				})

				It("keeps the state as retiring", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRetiring))
				})
			})

			Context("when the current state is running", func() {
				BeforeEach(func() {
					atcWorker.State = string(dbng.WorkerStateRunning)
				})

				It("keeps the state as running", func() {
					foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
					Expect(err).NotTo(HaveOccurred())

					Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRunning))
				})
			})

			Context("when the current state is stalled", func() {
				var (
					unresponsiveWorker dbng.Worker
					stalledWorkerNames []string
					err                error
				)

				JustBeforeEach(func() {
					unresponsiveWorker, err = workerFactory.SaveWorker(atcWorker, -5*time.Minute)
					Expect(err).NotTo(HaveOccurred())

					stalledWorkerNames, err = workerLifecycle.StallUnresponsiveWorkers()
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
					Expect(foundWorker.State()).To(Equal(dbng.WorkerStateRunning))
				})
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.HeartbeatWorker(atcWorker, ttl)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})
})
