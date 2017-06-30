package migration_test

import (
	"database/sql"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/migration"
	"github.com/concourse/atc/db/migrations"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddWorkerResourceCacheToContainers", func() {
	var dbConn *sql.DB
	var err error

	var migrator migration.Migrator
	var migrationsBefore []migration.Migrator

	BeforeEach(func() {
		migrator = migrations.AddWorkerResourceCacheToContainers
		migrationsBefore = []migration.Migrator{}
		for i, m := range migrations.New(db.NewNoEncryption()) {
			if i > 144 { // unfortunately you cannot compare functions
				break
			}

			migrationsBefore = append(migrationsBefore, m)
		}
	})

	Context("when there no existing resources", func() {
		BeforeEach(func() {
			dbConn, err = migration.Open(
				"postgres",
				postgresRunner.DataSourceName(),
				migrationsBefore,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := dbConn.Close()
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are containers for resource cache", func() {
			var containerID int
			var workerBaseResourceTypeID int

			BeforeEach(func() {
				var baseResourceTypeID int
				err := dbConn.QueryRow(`
					INSERT INTO base_resource_types (name) VALUES ($1) RETURNING id
				`, "docker-image").Scan(&baseResourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				_, err = dbConn.Exec(`
					INSERT INTO workers (name) VALUES ($1)
				`, "some-worker")
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO worker_base_resource_types (worker_name, base_resource_type_id, image, version) VALUES ($1, $2, $3, $4) RETURNING id
				`, "some-worker", baseResourceTypeID, "some-image", "some-version").Scan(&workerBaseResourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				var resourceConfigID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (base_resource_type_id, source_hash) VALUES ($1, $2) RETURNING id
				`, baseResourceTypeID, "some-source-hash").Scan(&resourceConfigID)
				Expect(err).NotTo(HaveOccurred())

				var resourceCacheID int
				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, resourceConfigID, "some-version", "some-params-hash").Scan(&resourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO resource_configs (resource_cache_id, source_hash) VALUES ($1, $2) RETURNING id
				`, resourceCacheID, "some-source-hash").Scan(&resourceConfigID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO resource_caches (resource_config_id, version, params_hash) VALUES ($1, $2, $3) RETURNING id
				`, resourceConfigID, "some-version", "some-params-hash").Scan(&resourceCacheID)
				Expect(err).NotTo(HaveOccurred())

				err = dbConn.QueryRow(`
					INSERT INTO containers (handle, type, step_name, worker_name, resource_cache_id) VALUES ($1, $2, $3, $4, $5) RETURNING id
				`, "some-handle", "get", "some-step", "some-worker", resourceCacheID).Scan(&containerID)
				Expect(err).NotTo(HaveOccurred())
			})

			It("migrates to worker_base_resource_type", func() {
				err := dbConn.Close()
				Expect(err).NotTo(HaveOccurred())

				migrationsToRun := append(migrationsBefore, migrator)

				dbConn, err = migration.Open(
					"postgres",
					postgresRunner.DataSourceName(),
					migrationsToRun,
				)
				Expect(err).NotTo(HaveOccurred())

				var migratedWorkerBaseResourceTypeID int
				err = dbConn.QueryRow(`
					SELECT worker_resource_cache_id FROM containers WHERE id=$1
				`, containerID).Scan(&migratedWorkerBaseResourceTypeID)
				Expect(err).NotTo(HaveOccurred())

				Expect(migratedWorkerBaseResourceTypeID).To(Equal(workerBaseResourceTypeID))
			})
		})
	})
})
