package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migrate existing job configs into job pipes", func() {
	const preMigrationVersion = 1573243444
	const postMigrationVersion = 1574452410

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates the job configs for all the jobs into the job pipes table", func() {
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

			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT job_id, resource_id, passed_job_id FROM job_pipes`)
			Expect(err).NotTo(HaveOccurred())

			type jobPipe struct {
				jobID      int
				resourceID int
				passedJob  int
			}

			var jobPipes []jobPipe
			for rows.Next() {
				jp := jobPipe{}
				var passedJobID sql.NullInt64

				err := rows.Scan(&jp.jobID, &jp.resourceID, &passedJobID)
				Expect(err).NotTo(HaveOccurred())

				if passedJobID.Valid {
					jp.passedJob = int(passedJobID.Int64)
				}

				jobPipes = append(jobPipes, jp)
			}

			_ = db.Close()

			expectedJobPipes := []jobPipe{
				{
					jobID:      1,
					resourceID: resource1ID,
				},
				{
					jobID:      2,
					resourceID: resource1ID,
					passedJob:  1,
				},
				{
					jobID:      2,
					resourceID: resource2ID,
				},
				{
					jobID:      3,
					resourceID: resource1ID,
					passedJob:  1,
				},
				{
					jobID:      3,
					resourceID: resource1ID,
					passedJob:  2,
				},
				{
					jobID:      3,
					resourceID: resource2ID,
					passedJob:  2,
				},
				{
					jobID:      5,
					resourceID: resource4ID,
				},
			}

			Expect(jobPipes).To(HaveLen(7))
			Expect(jobPipes).To(ConsistOf(expectedJobPipes))
		})
	})

	Context("Down", func() {
		It("truncates the job pipes table", func() {
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
				INSERT INTO job_pipes(job_id, resource_id, passed_job_id) VALUES
				(1, 1, NULL),
				(2, 1, 1),
				(2, 2, NULL),
				(3, 1, 1),
				(3, 1, 2),
				(3, 2, 2),
				(5, 4, NULL)
			`)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			var rowNumber int
			err = db.QueryRow(`SELECT COUNT(*) FROM job_pipes`).Scan(&rowNumber)
			Expect(err).NotTo(HaveOccurred())

			_ = db.Close()

			Expect(rowNumber).To(BeZero())
		})
	})
})
