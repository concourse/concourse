package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add resource pinned version", func() {
	const preMigrationVersion = 1537546150
	const postMigrationVersion = 1538408345

	var (
		db *sql.DB
	)

	setupResourceVersions := func(db *sql.DB) {
		_, err := db.Exec(`
			INSERT INTO base_resource_types(id, name) VALUES
			(1, 'some-type')
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`
			INSERT INTO resource_configs(id, base_resource_type_id, source_hash) VALUES
			(1, 1, 'some-source'),
			(2, 1, 'another-source')
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`
			INSERT INTO resources(id, pipeline_id, resource_config_id, name, config, active, paused) VALUES
			(1, 1, 1, 'resource1', '{}', true, true),
			(2, 1, 1, 'resource2', '{}', true, false),
			(3, 1, 2, 'resource3', '{}', true, true)
			`)
		Expect(err).NotTo(HaveOccurred())

		_, err = db.Exec(`
			INSERT INTO resource_config_versions(id, resource_config_id, check_order, version, version_md5, metadata) VALUES
			(1, 1, 1, '{"version": "1"}', 'abcde', 'null'),
			(2, 1, 4, '{"version": "4"}', 'bcdef', 'null'),
			(3, 2, 2, '{"version": "2"}', 'cdefg', 'null')
			`)
	}

	Context("Up", func() {
		It("creates a job tag for each job within every pipeline group", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupResourceVersions(db)

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT id, api_pinned_version FROM resources`)
			Expect(err).NotTo(HaveOccurred())

			pinnedVersions := make(map[int]string)

			for rows.Next() {
				var id int
				var version sql.NullString

				err := rows.Scan(&id, &version)
				Expect(err).NotTo(HaveOccurred())

				if version.Valid {
					pinnedVersions[id] = version.String
				}
			}

			_ = db.Close()

			Expect(pinnedVersions).To(HaveLen(2))
			Expect(pinnedVersions[1]).To(Equal(`{"version": "4"}`))
			Expect(pinnedVersions[3]).To(Equal(`{"version": "2"}`))
		})
	})
})
