package migration_test

import (
	"database/sql"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add global resource versions", func() {
	const preMigrationVersion = 1537196857
	const postMigrationVersion = 1537546150

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates disabled resource versions to new resource disabled versions table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupResource(db)
			setupVersionedResources(db)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			for i := 1; i <= 3; i++ {
				_, err := db.Exec("INSERT INTO resource_config_versions(version, version_md5, resource_config_id, check_order) VALUES($1::jsonb, md5($1::text), 1, $2)", fmt.Sprintf(`{"version": "v%d"}`, i), i)
				Expect(err).NotTo(HaveOccurred())
			}

			rows, err := db.Query(`SELECT d.resource_id, v.version FROM resource_disabled_versions d, resource_config_versions v WHERE d.version_md5 = v.version_md5`)
			Expect(err).NotTo(HaveOccurred())

			type disabledVersions struct {
				resourceID int
				version    string
			}

			dvs := []disabledVersions{}
			for rows.Next() {
				dv := disabledVersions{}

				err := rows.Scan(&dv.resourceID, &dv.version)
				Expect(err).NotTo(HaveOccurred())

				dvs = append(dvs, dv)
			}

			_ = db.Close()

			Expect(dvs).To(HaveLen(1))
			Expect(dvs[0].resourceID).To(Equal(1))
			Expect(dvs[0].version).To(Equal(`{"version": "v3"}`))
		})
	})

	Context("Down", func() {
		It("saves all versions (disabled and enabled) into the versioned resources table", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			setup(db)
			setupResource(db)
			setupVersionedResources(db)

			// Insert three versions into resource config versions table
			for i := 1; i <= 3; i++ {
				_, err := db.Exec("INSERT INTO resource_config_versions(version, version_md5, resource_config_id, check_order) VALUES($1::jsonb, md5($1::text), 1, $2)", fmt.Sprintf(`{"version": "v%d"}`, i), i)
				Expect(err).NotTo(HaveOccurred())
			}

			// Disable the second and third resource version
			_, err := db.Exec("INSERT INTO resource_disabled_versions(version_md5, resource_id) VALUES(md5($1::text), 1)", fmt.Sprintf(`{"version": "v%d"}`, 2))
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec("INSERT INTO resource_disabled_versions(version_md5, resource_id) VALUES(md5($1::text), 1)", fmt.Sprintf(`{"version": "v%d"}`, 3))
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			rows, err := db.Query(`SELECT v.resource_id, v.version, v.type, v.enabled FROM versioned_resources v`)
			Expect(err).NotTo(HaveOccurred())

			type versionedResource struct {
				resourceID int
				version    string
				enabled    bool
				type_      string
			}

			vs := []versionedResource{}
			for rows.Next() {
				v := versionedResource{}

				err := rows.Scan(&v.resourceID, &v.version, &v.type_, &v.enabled)
				Expect(err).NotTo(HaveOccurred())

				vs = append(vs, v)
			}

			db.Close()

			Expect(vs).To(HaveLen(3))

			desiredVersionedResources := []versionedResource{
				{
					resourceID: 1,
					version:    `{"version": "v1"}`,
					type_:      "some-type",
					enabled:    true,
				},
				{
					resourceID: 1,
					version:    `{"version": "v2"}`,
					type_:      "some-type",
					enabled:    false,
				},
				{
					resourceID: 1,
					version:    `{"version": "v3"}`,
					type_:      "some-type",
					enabled:    false,
				},
			}

			Expect(vs).To(ConsistOf(desiredVersionedResources))

		})
	})
})

func setupResource(db *sql.DB) {
	_, err := db.Exec("INSERT INTO base_resource_types(name) VALUES('some-type')")
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec("INSERT INTO resource_configs(source_hash, base_resource_type_id) VALUES('some-source', 1)")
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO resources(name, pipeline_id, config, active, resource_config_id) VALUES('some-resource', 1, '{"type": "some-type"}', true, 1)`)
	Expect(err).NotTo(HaveOccurred())
}

func setupVersionedResources(db *sql.DB) {
	// Insert two enabled versions into the versioned resources table
	for i := 1; i <= 2; i++ {
		_, err := db.Exec("INSERT INTO versioned_resources(version, metadata, type, enabled, resource_id, check_order) VALUES($1, 'some-metadata', 'some-type', true, 1, $2)", fmt.Sprintf(`{"version": "v%d"}`, i), i)
		Expect(err).NotTo(HaveOccurred())
	}

	// Insert a disabled version into the versioned resources table
	_, err := db.Exec(`INSERT INTO versioned_resources(version, metadata, type, enabled, resource_id, check_order) VALUES('{"version": "v3"}', 'some-metadata', 'some-type', false, 1, 3)`)
	Expect(err).NotTo(HaveOccurred())
}
