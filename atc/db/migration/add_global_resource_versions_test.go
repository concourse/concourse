package migration_test

import (
	"database/sql"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
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

			Expect(dvs).To(HaveLen(2))
			Expect(dvs[0].resourceID).To(Equal(1))
			Expect(dvs[0].version).To(Equal(`{"version": "v3"}`))
		})

		It("migrates the build inputs into the new build resource config version inputs table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupResource(db)
			setupVersionedResources(db)
			setupBuilds(db)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT build_id, version_md5, resource_id, name FROM build_resource_config_version_inputs`)
			Expect(err).NotTo(HaveOccurred())

			type buildInput struct {
				buildID    int
				versionMD5 string
				resourceID int
				name       string
			}

			buildInputs := []buildInput{}
			for rows.Next() {
				bi := buildInput{}

				err := rows.Scan(&bi.buildID, &bi.versionMD5, &bi.resourceID, &bi.name)
				Expect(err).NotTo(HaveOccurred())

				buildInputs = append(buildInputs, bi)
			}

			rows, err = db.Query(`SELECT id, md5(version) FROM versioned_resources`)
			Expect(err).NotTo(HaveOccurred())

			versions := map[int]string{}
			for rows.Next() {
				var id int
				var version string

				err := rows.Scan(&id, &version)
				Expect(err).NotTo(HaveOccurred())

				versions[id] = version
			}

			_ = db.Close()

			actualBuildInputs := []buildInput{
				{
					buildID:    1,
					versionMD5: versions[1],
					resourceID: 1,
					name:       "build_input1",
				},
				{
					buildID:    1,
					versionMD5: versions[2],
					resourceID: 1,
					name:       "build_input2",
				},
				{
					buildID:    2,
					versionMD5: versions[1],
					resourceID: 1,
					name:       "build_input3",
				},
				{
					buildID:    3,
					versionMD5: versions[1],
					resourceID: 1,
					name:       "build_input4",
				},
				{
					buildID:    4,
					versionMD5: versions[4],
					resourceID: 2,
					name:       "build_input5",
				},
			}

			Expect(buildInputs).To(HaveLen(5))
			Expect(buildInputs).To(ConsistOf(actualBuildInputs))
		})

		It("migrates the build outputs into the new build resource config version outputs table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupResource(db)
			setupVersionedResources(db)
			setupBuildOutputs(db)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT build_id, version_md5, resource_id, name FROM build_resource_config_version_outputs`)
			Expect(err).NotTo(HaveOccurred())

			type buildOutput struct {
				buildID    int
				versionMD5 string
				resourceID int
				name       string
			}

			buildOutputs := []buildOutput{}
			for rows.Next() {
				bo := buildOutput{}

				err := rows.Scan(&bo.buildID, &bo.versionMD5, &bo.resourceID, &bo.name)
				Expect(err).NotTo(HaveOccurred())

				buildOutputs = append(buildOutputs, bo)
			}

			rows, err = db.Query(`SELECT id, md5(version) FROM versioned_resources`)
			Expect(err).NotTo(HaveOccurred())

			versions := map[int]string{}
			for rows.Next() {
				var id int
				var version string

				err := rows.Scan(&id, &version)
				Expect(err).NotTo(HaveOccurred())

				versions[id] = version
			}

			_ = db.Close()

			actualBuildOutputs := []buildOutput{
				{
					buildID:    1,
					versionMD5: versions[2],
					resourceID: 1,
					name:       "some-resource",
				},
				{
					buildID:    2,
					versionMD5: versions[1],
					resourceID: 1,
					name:       "some-resource",
				},
				{
					buildID:    3,
					versionMD5: versions[1],
					resourceID: 1,
					name:       "some-resource",
				},
				{
					buildID:    4,
					versionMD5: versions[4],
					resourceID: 2,
					name:       "some-other-resource",
				},
			}

			Expect(buildOutputs).To(HaveLen(4))
			Expect(buildOutputs).To(ConsistOf(actualBuildOutputs))
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

			Expect(vs).To(HaveLen(6))

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
				{
					resourceID: 2,
					version:    `{"version": "v1"}`,
					type_:      "some-type",
					enabled:    true,
				},
				{
					resourceID: 2,
					version:    `{"version": "v2"}`,
					type_:      "some-type",
					enabled:    true,
				},
				{
					resourceID: 2,
					version:    `{"version": "v3"}`,
					type_:      "some-type",
					enabled:    true,
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

	_, err = db.Exec(`INSERT INTO resources(name, pipeline_id, config, active, resource_config_id) VALUES('some-other-resource', 2, '{"type": "some-type"}', true, 1)`)
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

	// Insert another version into the versioned resources table for pipeline 2
	_, err = db.Exec(`INSERT INTO versioned_resources(version, metadata, type, enabled, resource_id, check_order) VALUES('{"version": "v1"}', 'some-metadata', 'some-type', false, 2, 1)`)
	Expect(err).NotTo(HaveOccurred())
}

func setupBuilds(db *sql.DB) {
	_, err := db.Exec(`
				INSERT INTO builds(id, name, status, job_id, team_id, pipeline_id) VALUES
					(1, 'build1', 'succeeded', 1, 1, 1),
					(2, 'build2', 'succeeded', 1, 1, 1),
					(3, 'build3', 'started', 2, 1, 1),
					(4, 'build4', 'pending', 4, 1, 2)
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO build_inputs(build_id, versioned_resource_id, name) VALUES
					(1, 1, 'build_input1'),
					(1, 2, 'build_input2'),
					(2, 1, 'build_input3'),
					(3, 1, 'build_input4'),
					(3, 1, 'build_input4'),
					(4, 4, 'build_input5')
			`)
	Expect(err).NotTo(HaveOccurred())
}

func setupBuildOutputs(db *sql.DB) {
	_, err := db.Exec(`
				INSERT INTO builds(id, name, status, job_id, team_id, pipeline_id) VALUES
					(1, 'build1', 'succeeded', 1, 1, 1),
					(2, 'build2', 'succeeded', 1, 1, 1),
					(3, 'build3', 'started', 2, 1, 1),
					(4, 'build4', 'pending', 4, 1, 2)
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO build_outputs(build_id, versioned_resource_id) VALUES
					(1, 2),
					(2, 1),
					(3, 1),
					(3, 1),
					(4, 4)
			`)
	Expect(err).NotTo(HaveOccurred())
}
