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

			rows, err := db.Query(`SELECT build_id, version_md5, resource_id, job_id, name FROM successful_build_versions`)
			Expect(err).NotTo(HaveOccurred())

			type successfulBuildVersion struct {
				buildID    int
				versionMD5 string
				resourceID int
				jobID      int
				name       string
			}

			successfulBuildVersions := []successfulBuildVersion{}
			for rows.Next() {
				sb := successfulBuildVersion{}

				err := rows.Scan(&sb.buildID, &sb.versionMD5, &sb.resourceID, &sb.jobID, &sb.name)
				Expect(err).NotTo(HaveOccurred())

				successfulBuildVersions = append(successfulBuildVersions, sb)
			}

			_ = db.Close()

			actualSuccessfulBuildVersions := []successfulBuildVersion{
				{
					buildID:    1,
					versionMD5: "v1",
					resourceID: 1,
					jobID:      1,
					name:       "build_input1",
				},
				{
					buildID:    1,
					versionMD5: "v1",
					resourceID: 1,
					jobID:      1,
					name:       "build_input2",
				},
				{
					buildID:    2,
					versionMD5: "v2",
					resourceID: 1,
					jobID:      1,
					name:       "build_input1",
				},
				{
					buildID:    5,
					versionMD5: "v2",
					resourceID: 2,
					jobID:      1,
					name:       "build_input4",
				},
				{
					buildID:    1,
					versionMD5: "v1",
					resourceID: 1,
					jobID:      1,
					name:       "build_output1",
				},
				{
					buildID:    2,
					versionMD5: "v3",
					resourceID: 1,
					jobID:      1,
					name:       "build_output1",
				},
				{
					buildID:    2,
					versionMD5: "v2",
					resourceID: 3,
					jobID:      1,
					name:       "build_output1",
				},
			}

			Expect(len(successfulBuildVersions)).To(Equal(len(actualSuccessfulBuildVersions)))
			Expect(successfulBuildVersions).To(ConsistOf(actualSuccessfulBuildVersions))
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
					(1, 1, 'v1', 'build_input1'),
					(1, 1, 'v1', 'build_input1'),
					(1, 1, 'v1', 'build_input2'),
					(2, 1, 'v2', 'build_input1'),
					(3, 1, 'v3', 'build_input1'),
					(4, 3, 'v1', 'build_input3'),
					(5, 2, 'v1', 'build_input4'),
			`)
	Expect(err).NotTo(HaveOccurred())
}

func setupSuccessfulBuildsOutputs(db *sql.DB) {
	_, err := db.Exec(`
				INSERT INTO build_resource_config_version_outputs(build_id, resource_id, version_md5, name) VALUES
					(1, 1, 'v1', 'build_output1'),
					(2, 1, 'v3', 'build_output1'),
					(2, 3, 'v2', 'build_output2'),
					(3, 1, 'v3', 'build_output1'),
					(4, 2, 'v1', 'build_output3'),
			`)
	Expect(err).NotTo(HaveOccurred())
}
