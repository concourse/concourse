package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add job tags", func() {
	const preMigrationVersion = 1522176230
	const postMigrationVersion = 1522178770

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("creates a job tag for each job within every pipeline group", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT id, tags FROM jobs`)
			Expect(err).NotTo(HaveOccurred())

			jobTags := make(map[int]string)

			for rows.Next() {
				var id int
				var jobTag string

				err := rows.Scan(&id, &jobTag)
				Expect(err).NotTo(HaveOccurred())

				jobTags[id] = jobTag
			}

			_ = db.Close()

			Expect(jobTags[1]).To(Equal("{group1}"))
			Expect(jobTags[2]).To(Equal("{group1,group2}"))
			Expect(jobTags[3]).To(Equal("{group2}"))
			Expect(jobTags[4]).To(Equal("{group2}"))
		})
	})
})

func setup(db *sql.DB) {
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
					(1, 1, 'job1', '{"name":"job1"}'),
					(2, 1, 'job2', '{"name":"job2"}'),
					(3, 1, 'job3', '{"name":"job3"}'),
					(4, 2, 'job1', '{"name":"job1"}')
			`)
	Expect(err).NotTo(HaveOccurred())
}
