package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add successful build outputs", func() {
	const preMigrationVersion = 1564686443
	const postMigrationVersion = 1564686445

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates the build inputs and outputs into the new successful build outputs table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)
			setupSuccessfulBuildsResource(db)
			setupSuccessfulBuildsInputs(db)
			setupSuccessfulBuildsOutputs(db)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT build_id, job_id, outputs FROM successful_build_outputs`)
			Expect(err).NotTo(HaveOccurred())

			type successfulBuildOutput struct {
				buildID int
				outputs string
				jobID   int
			}

			successfulBuildOutputs := []successfulBuildOutput{}
			for rows.Next() {
				sb := successfulBuildOutput{}

				err := rows.Scan(&sb.buildID, &sb.jobID, &sb.outputs)
				Expect(err).NotTo(HaveOccurred())

				successfulBuildOutputs = append(successfulBuildOutputs, sb)
			}

			_ = db.Close()

			actualSuccessfulBuildOutputs := []successfulBuildOutput{
				{
					buildID: 1,
					jobID:   1,
					outputs: `{"1": ["v1"]}`,
				},
				{
					buildID: 2,
					jobID:   1,
					outputs: `{"1": ["v3", "v2"], "3": ["v2"]}`,
				},
				{
					buildID: 5,
					jobID:   1,
					outputs: `{"2": ["v1"]}`,
				},
			}

			Expect(len(successfulBuildOutputs)).To(Equal(len(actualSuccessfulBuildOutputs)))
			Expect(successfulBuildOutputs).To(ConsistOf(actualSuccessfulBuildOutputs))
		})
	})
})

func setupSuccessfulBuildsResource(db *sql.DB) {
	_, err := db.Exec(`INSERT INTO resources(name, type, pipeline_id, config, active) VALUES('some-resource', 'some-type', 1, '{"type": "some-type"}', true)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO resources(name, type, pipeline_id, config, active) VALUES('some-other-resource', 'some-type', 2, '{"type": "some-type"}', true)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO resources(name, type, pipeline_id, config, active) VALUES('some-resource-2', 'some-type', 1, '{"type": "some-type"}', true)`)
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
				INSERT INTO build_resource_config_version_inputs(build_id, resource_id, version_md5, name, first_occurrence) VALUES
					(1, 1, 'v1', 'build_input1', true),
					(1, 1, 'v1', 'build_input2', true),
					(2, 1, 'v2', 'build_input1', false),
					(3, 1, 'v3', 'build_input1', true),
					(4, 3, 'v1', 'build_input3', false),
					(5, 2, 'v1', 'build_input4', true)
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
					(4, 2, 'v1', 'build_output3')
			`)
	Expect(err).NotTo(HaveOccurred())
}
