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
			Expect(savedWorker.GardenAddr).To(Equal("some-garden-addr"))
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
})
