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
		dbConn        dbng.Conn
		workerFactory dbng.WorkerFactory

		atcWorker atc.Worker
	)

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = dbng.Wrap(postgresRunner.Open())
		workerFactory = dbng.NewWorkerFactory(dbConn)

		atcWorker = atc.Worker{
			GardenAddr:       "some-garden-addr",
			BaggageclaimURL:  "some-bc-url",
			HTTPProxyURL:     "some-http-proxy-url",
			HTTPSProxyURL:    "some-https-proxy-url",
			NoProxy:          "some-no-proxy",
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
			StartTime: 55,
		}
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("SaveWorker", func() {
		It("saves worker", func() {
			savedWorker, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())
			Expect(savedWorker.Name).To(Equal("some-name"))
			Expect(*savedWorker.GardenAddr).To(Equal("some-garden-addr"))
			Expect(savedWorker.State).To(Equal(dbng.WorkerStateRunning))
		})

		It("saves worker resource types as base resource types", func() {
			_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			tx, err := dbConn.Begin()
			Expect(err).NotTo(HaveOccurred())
			defer tx.Rollback()

			var count int
			err = psql.Select("count(*)").
				From("worker_base_resource_types").
				Where(sq.Eq{"worker_name": "some-name"}).
				RunWith(tx).
				QueryRow().Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(2))
		})

		It("removes old worker resource type", func() {
			_, err := workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			atcWorker.ResourceTypes = []atc.WorkerResourceType{
				{
					Type:    "other-resource-type",
					Image:   "other-image",
					Version: "other-version",
				},
			}

			_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
			Expect(err).NotTo(HaveOccurred())

			tx, err := dbConn.Begin()
			Expect(err).NotTo(HaveOccurred())
			defer tx.Rollback()

			var count int
			err = psql.Select("count(*)").
				From("worker_base_resource_types").
				Where(sq.Eq{"worker_name": "some-name"}).
				RunWith(tx).
				QueryRow().Scan(&count)
			Expect(err).NotTo(HaveOccurred())
			Expect(count).To(Equal(1))
		})
	})

	Describe("GetWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds the worker", func() {
				foundWorker, found, err := workerFactory.GetWorker("some-name")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(foundWorker.Name).To(Equal("some-name"))
				Expect(*foundWorker.GardenAddr).To(Equal("some-garden-addr"))
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateRunning))
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
		Context("when there are workers", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())

				atcWorker.Name = "some-new-worker"
				atcWorker.GardenAddr = "some-other-garden-addr"
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("finds them without error", func() {
				workers, err := workerFactory.Workers()
				addr := "some-garden-addr"
				otherAddr := "some-other-garden-addr"
				Expect(err).NotTo(HaveOccurred())
				Expect(len(workers)).To(Equal(2))
				Expect(workers).To(ConsistOf([]*dbng.Worker{
					&dbng.Worker{
						GardenAddr: &addr,
						Name:       "some-name",
						State:      "running",
					},
					&dbng.Worker{
						GardenAddr: &otherAddr,
						Name:       "some-new-worker",
						State:      "running",
					},
				}))
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

	Describe("StallWorker", func() {
		Context("when the worker is present", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `stalled`", func() {
				foundWorker, err := workerFactory.StallWorker("some-name")
				Expect(err).NotTo(HaveOccurred())

				Expect(foundWorker.Name).To(Equal("some-name"))
				Expect(foundWorker.GardenAddr).To(BeNil())
				Expect(foundWorker.State).To(Equal(dbng.WorkerStateStalled))
			})
		})

		Context("when the worker is not present", func() {
			It("returns an error", func() {
				foundWorker, err := workerFactory.StallWorker("some-name")
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(dbng.ErrWorkerNotPresent))
				Expect(foundWorker).To(BeNil())
			})
		})
	})

	Describe("StallUnresponsiveWorkers", func() {
		Context("when the worker has heartbeated recently", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, 5*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("leaves the worker alone", func() {
				stalledWorkers, err := workerFactory.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())
				Expect(stalledWorkers).To(BeEmpty())
			})
		})

		Context("when the worker has not heartbeated recently", func() {
			BeforeEach(func() {
				_, err = workerFactory.SaveWorker(atcWorker, -1*time.Minute)
				Expect(err).NotTo(HaveOccurred())
			})

			It("marks the worker as `stalled`", func() {
				stalledWorkers, err := workerFactory.StallUnresponsiveWorkers()
				Expect(err).NotTo(HaveOccurred())

				Expect(len(stalledWorkers)).To(Equal(1))
				Expect(stalledWorkers[0].GardenAddr).To(BeNil())
				Expect(stalledWorkers[0].Name).To(Equal("some-name"))
				Expect(stalledWorkers[0].State).To(Equal(dbng.WorkerStateStalled))
			})
		})
	})
})
