package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add successful build versions", func() {
	const preMigrationVersion = 1560050191
	const postMigrationVersion = 1560197908

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates the build inputs and outputs into the new successful build versions table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupSuccessfulBuildsResource(db)
			setupSuccessfulBuildsInputs(db)
			setupSuccessfulBuildsOutputs(db)
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
	})
})

func setupSuccessfulBuildsResource(db *sql.DB) {
	_, err := db.Exec(`INSERT INTO resources(name, pipeline_id, config, active) VALUES('some-resource', 1, '{"type": "some-type"}', true)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO resources(name, pipeline_id, config, active) VALUES('some-other-resource', 2, '{"type": "some-type"}', true)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO resources(name, pipeline_id, config, active) VALUES('some-resource-2', 1, '{"type": "some-type"}', true)`)
	Expect(err).NotTo(HaveOccurred())
}

func setupSuccessfulBuildsInputs(db *sql.DB) {
	_, err := db.Exec(`
				INSERT INTO builds(id, name, status, job_id, team_id, pipeline_id) VALUES
					(1, 'build1', 'succeeded', 1, 1, 1),
					(2, 'build2', 'succeeded', 1, 1, 1),
					(3, 'build3', 'started', 2, 1, 1),
					(4, 'build4', 'pending', 4, 1, 2),
					(5, 'build5', 'succeeded', 1, 1, 2)
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO build_resource_config_version_inputs(build_id, resource_id, version_md5, name) VALUES
					(1, 1, 'build_input1'),
					(1, 1, 'build_input2'),
					(2, 1, 'build_input3'),
					(3, 1, 'build_input4'),
					(3, 1, 'build_input4'),
					(4, 4, 'build_input5'),
					(5, 2, 'build_input6'),
			`)
	Expect(err).NotTo(HaveOccurred())
}

func setupSuccessfulBuildsOutputs(db *sql.DB) {
	_, err := db.Exec(`
				INSERT INTO builds(id, name, status, job_id, team_id, pipeline_id) VALUES
					(6, 'build1', 'succeeded', 1, 1, 1),
					(7, 'build2', 'succeeded', 1, 1, 1),
					(8, 'build3', 'started', 2, 1, 1),
					(9, 'build4', 'pending', 4, 1, 2)
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO build_resource_config_version_outputs(build_id, resource_id, version_md5, name) VALUES
					(1, 3, 'some-resource-2', 'build_output1'),
					(2, 1),
					(3, 1),
					(3, 1),
					(4, 4)
			`)
	Expect(err).NotTo(HaveOccurred())
}
