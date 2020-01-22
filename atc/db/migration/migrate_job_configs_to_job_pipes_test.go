package migration_test

import (
	"database/sql"

	"github.com/concourse/concourse/atc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate existing job configs into job pipes", func() {
	const preMigrationVersion = 1579713198
	const postMigrationVersion = 1579713199

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates the job configs for all the jobs into the job inputs table, job outputs table and the new config fields in the jobs table", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name, groups) VALUES
			(1, 1, 'pipeline1', '[{"name":"group1","jobs":["job1","job2"]},{"name":"group2","jobs":["job2","job3"]}]'),
			(2, 1, 'pipeline2', '[{"name":"group2","jobs":["job1"]}]')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
				INSERT INTO jobs(id, pipeline_id, name, active, config) VALUES
				(1, 1, 'job1', true, '{"name":"job1","plan":[{"get":"resource-1"}]}'),
				(2, 1, 'job2', true, '{"name":"job2","public":true,"disable_manual_trigger":true,"serial_groups":["serial-1","serial-2"],"plan":[{"get":"resource-1","passed":["job1"],"trigger":true,"version":"latest"},{"get":"resource-2"}]}'),
				(3, 1, 'job3', true, '{"name":"job3","max_in_flight":5,"disable_manual_trigger":false,"plan":[{"get":"res1","resource":"resource-1","passed":["job1","job2"],"version":{"ver":"1"}},{"get":"resource-2","passed":["job2"],"trigger":false}]}'),
				(4, 2, 'job1', true, '{"name":"job1","serial":true,"plan":[{"put":"resource-3"}]}'),
				(5, 2, 'job2', true, '{"name":"job2","public":false,"serial_groups":["serial-1"],"plan":[{"get":"resource-1","version":"every"},{"put":"res-1","resource":"resource-1"}]}'),
				(6, 2, 'job3', false, '{"name":"job3","serial_groups":["serial-1"],"plan":[{"get":"resource-2"},{"put":"resource-1"}]}')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO jobs_serial_groups(serial_group, job_id) VALUES
			('serial-group-1', 1),
			('serial-1', 2),
			('serial-5', 5)
			`)
			Expect(err).NotTo(HaveOccurred())

			var resource1ID, resource2ID, resource3ID, resource4ID int

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource1ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-2', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource2ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-3', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource3ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource4ID)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			type jobConfigFields struct {
				jobID                int
				public               bool
				disableManualTrigger bool
				maxInFlight          int
			}

			rows, err := db.Query(`SELECT id, public, disable_manual_trigger, max_in_flight FROM jobs WHERE active = true`)
			Expect(err).NotTo(HaveOccurred())

			var jobs []jobConfigFields
			for rows.Next() {
				job := jobConfigFields{}

				err := rows.Scan(&job.jobID, &job.public, &job.disableManualTrigger, &job.maxInFlight)
				Expect(err).NotTo(HaveOccurred())

				jobs = append(jobs, job)
			}

			expectedJobConfigFields := []jobConfigFields{
				{
					jobID: 1,
				},
				{
					jobID:                2,
					public:               true,
					disableManualTrigger: true,
					maxInFlight:          1,
				},
				{
					jobID:                3,
					disableManualTrigger: false,
					maxInFlight:          5,
				},
				{
					jobID:       4,
					maxInFlight: 1,
				},
				{
					jobID:       5,
					public:      false,
					maxInFlight: 1,
				},
			}

			Expect(jobs).To(HaveLen(5))
			Expect(jobs).To(ConsistOf(expectedJobConfigFields))

			rows, err = db.Query(`SELECT name, job_id, resource_id, passed_job_id, trigger, version FROM job_inputs`)
			Expect(err).NotTo(HaveOccurred())

			type jobInput struct {
				name       string
				jobID      int
				resourceID int
				passedJob  int
				trigger    bool
				version    *atc.VersionConfig
			}

			var jobInputs []jobInput
			for rows.Next() {
				ji := jobInput{}
				var passedJobID sql.NullInt64
				var versionString sql.NullString

				err := rows.Scan(&ji.name, &ji.jobID, &ji.resourceID, &passedJobID, &ji.trigger, &versionString)
				Expect(err).NotTo(HaveOccurred())

				if versionString.Valid {
					version := &atc.VersionConfig{}
					err = version.UnmarshalJSON([]byte(versionString.String))
					Expect(err).NotTo(HaveOccurred())

					ji.version = version
				}

				if passedJobID.Valid {
					ji.passedJob = int(passedJobID.Int64)
				}

				jobInputs = append(jobInputs, ji)
			}

			expectedJobInputs := []jobInput{
				{
					name:       "resource-1",
					jobID:      1,
					resourceID: resource1ID,
				},
				{
					name:       "resource-1",
					jobID:      2,
					resourceID: resource1ID,
					passedJob:  1,
					trigger:    true,
					version:    &atc.VersionConfig{Latest: true},
				},
				{
					name:       "resource-2",
					jobID:      2,
					resourceID: resource2ID,
				},
				{
					name:       "res1",
					jobID:      3,
					resourceID: resource1ID,
					passedJob:  1,
					version:    &atc.VersionConfig{Pinned: atc.Version{"ver": "1"}},
				},
				{
					name:       "res1",
					jobID:      3,
					resourceID: resource1ID,
					passedJob:  2,
					version:    &atc.VersionConfig{Pinned: atc.Version{"ver": "1"}},
				},
				{
					name:       "resource-2",
					jobID:      3,
					resourceID: resource2ID,
					passedJob:  2,
					trigger:    false,
				},
				{
					name:       "resource-1",
					jobID:      5,
					resourceID: resource4ID,
					version:    &atc.VersionConfig{Every: true},
				},
			}

			Expect(jobInputs).To(HaveLen(7))
			Expect(jobInputs).To(ConsistOf(expectedJobInputs))

			rows, err = db.Query(`SELECT name, job_id, resource_id FROM job_outputs`)
			Expect(err).NotTo(HaveOccurred())

			type jobOutput struct {
				name       string
				jobID      int
				resourceID int
			}

			var jobOutputs []jobOutput
			for rows.Next() {
				jo := jobOutput{}

				err := rows.Scan(&jo.name, &jo.jobID, &jo.resourceID)
				Expect(err).NotTo(HaveOccurred())

				jobOutputs = append(jobOutputs, jo)
			}

			expectedJobOutputs := []jobOutput{
				{
					name:       "resource-3",
					jobID:      4,
					resourceID: resource3ID,
				},
				{
					name:       "res-1",
					jobID:      5,
					resourceID: resource4ID,
				},
			}

			Expect(jobOutputs).To(HaveLen(2))
			Expect(jobOutputs).To(ConsistOf(expectedJobOutputs))

			rows, err = db.Query(`SELECT job_id, serial_group FROM jobs_serial_groups`)
			Expect(err).NotTo(HaveOccurred())

			type serialGroup struct {
				jobID       int
				serialGroup string
			}

			var serialGroups []serialGroup
			for rows.Next() {
				sg := serialGroup{}

				err := rows.Scan(&sg.jobID, &sg.serialGroup)
				Expect(err).NotTo(HaveOccurred())

				serialGroups = append(serialGroups, sg)
			}

			_ = db.Close()

			expectedSerialGroups := []serialGroup{
				{
					jobID:       2,
					serialGroup: "serial-1",
				},
				{
					jobID:       2,
					serialGroup: "serial-2",
				},
				{
					jobID:       3,
					serialGroup: "job3",
				},
				{
					jobID:       4,
					serialGroup: "job1",
				},
				{
					jobID:       5,
					serialGroup: "serial-1",
				},
			}

			Expect(serialGroups).To(HaveLen(5))
			Expect(serialGroups).To(ConsistOf(expectedSerialGroups))
		})
	})

	Context("Down", func() {
		It("truncates the job inputs and outputs table", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name, groups) VALUES
			(1, 1, 'pipeline1', '[{"name":"group1","jobs":["job1","job2"]},{"name":"group2","jobs":["job2","job3"]}]'),
			(2, 1, 'pipeline2', '[{"name":"group2","jobs":["job1"]}]')
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
				INSERT INTO jobs(id, pipeline_id, name, config) VALUES
				(1, 1, 'job1', '{"name":"job1","plan":[{"get":"resource-1"}]}'),
				(2, 1, 'job2', '{"name":"job2","plan":[{"get":"resource-1","passed":["job1"]},{"get":"resource-2"}]}'),
				(3, 1, 'job3', '{"name":"job3","plan":[{"get":"res1","resource":"resource-1","passed":["job1","job2"]},{"get":"resource-2","passed":["job2"]}]}'),
				(4, 2, 'job1', '{"name":"job1","plan":[{"put":"resource-3"}]}'),
				(5, 2, 'job2', '{"name":"job2","plan":[{"get":"resource-1"}]}')
			`)
			Expect(err).NotTo(HaveOccurred())

			var resource1ID, resource2ID, resource3ID, resource4ID int

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource1ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-2', 1, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource2ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-3', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource3ID)
			Expect(err).NotTo(HaveOccurred())

			err = db.QueryRow(`INSERT INTO resources(name, pipeline_id, config, active, type) VALUES('resource-1', 2, '{"type": "some-type"}', true, 'some-type') RETURNING id`).Scan(&resource4ID)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
				INSERT INTO job_inputs(name, job_id, resource_id, passed_job_id, trigger, version) VALUES
				('resource-1', 1, 1, NULL, false, NULL),
				('resource-1', 2, 1, 1, false, 'latest'),
				('resource-2', 2, 2, NULL, true, NULL),
				('res1', 3, 1, 1, false, 'every'),
				('res1', 3, 1, 2, false, '{"ver":"1"}'),
				('resource-2', 3, 2, 2, true, NULL),
				('resource-1', 5, 4, NULL, false, NULL)
			`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
				INSERT INTO job_outputs(name, job_id, resource_id) VALUES
				('resource-1', 1, 1),
				('resource-1', 2, 1),
				('resource-2', 2, 2)
			`)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			var inputsRowNumber int
			err = db.QueryRow(`SELECT COUNT(*) FROM job_inputs`).Scan(&inputsRowNumber)
			Expect(err).NotTo(HaveOccurred())

			var outputsRowNumber int
			err = db.QueryRow(`SELECT COUNT(*) FROM job_inputs`).Scan(&outputsRowNumber)
			Expect(err).NotTo(HaveOccurred())

			_ = db.Close()

			Expect(inputsRowNumber).To(BeZero())
			Expect(outputsRowNumber).To(BeZero())
		})
	})
})
