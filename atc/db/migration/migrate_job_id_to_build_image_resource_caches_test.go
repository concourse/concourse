package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate job ids to build image resource caches", func() {
	const preMigrationVersion = 1609958557
	const postMigrationVersion = 1609958558

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates job ids to build image resource caches", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)

			_, err := db.Exec(`
			INSERT INTO builds(name, job_id, status, team_id) VALUES
			('1',1,'started',1),
			('1',4,'started',1),
			('2',NULL,'started',1)
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO base_resource_types(name) VALUES ('base-type')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`INSERT INTO resource_configs(base_resource_type_id, source_hash) VALUES (1, 'hash')`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO resource_caches(resource_config_id, version, params_hash, version_md5) VALUES
			(1, '{"ver": "1"}', 'params1', 'ver1'),
			(1, '{"ver": "2"}', 'params2', 'ver2'),
			(1, '{"ver": "3"}', 'params3', 'ver3')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO build_image_resource_caches(build_id, resource_cache_id) VALUES
			(1, 1),
			(1, 2),
			(2, 2),
			(3, 3)
			`)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			type buildImageResourceCache struct {
				jobID           int
				buildID         int
				resourceCacheID int
			}

			rows, err := db.Query(`SELECT build_id, resource_cache_id, job_id FROM build_image_resource_caches`)
			Expect(err).NotTo(HaveOccurred())

			var caches []buildImageResourceCache
			for rows.Next() {
				cache := buildImageResourceCache{}

				var jobID sql.NullInt64
				err := rows.Scan(&cache.buildID, &cache.resourceCacheID, &jobID)
				Expect(err).NotTo(HaveOccurred())

				if jobID.Valid {
					cache.jobID = int(jobID.Int64)
				}

				caches = append(caches, cache)
			}

			expectedBuildImageResourceCaches := []buildImageResourceCache{
				{
					buildID:         1,
					jobID:           1,
					resourceCacheID: 1,
				},
				{
					buildID:         1,
					jobID:           1,
					resourceCacheID: 2,
				},
				{
					buildID:         2,
					jobID:           4,
					resourceCacheID: 2,
				},
				{
					buildID:         3,
					resourceCacheID: 3,
				},
			}

			db.Close()

			Expect(caches).To(HaveLen(4))
			Expect(caches).To(ConsistOf(expectedBuildImageResourceCaches))
		})
	})
})
