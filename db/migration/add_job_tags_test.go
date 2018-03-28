package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type JobTag struct {
	JobId int
	Tag   string
}

var _ = Describe("Add job tags", func() {
	const preMigrationVersion = 1522176230
	const postMigrationVersion = 1522178770

	var (
		db *sql.DB
	)

	Context("Down", func() {
		It("truncates all job tags", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			setup(db)

			_, err := db.Exec(`
				INSERT INTO job_tags(job_id, tag) VALUES
				(1, 'group1'),
				(2, 'group1'),
				(2, 'group2'),
				(3, 'group2'),
				(4, 'group2')
			`)
			Expect(err).NotTo(HaveOccurred())

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			jobTagsCount, err := countRows(db, "job_tags")
			Expect(err).NotTo(HaveOccurred())
			Expect(jobTagsCount).To(Equal(0))

			_ = db.Close()
		})
	})

	Context("Up", func() {
		It("creates a job tag for each job within every pipeline group", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			setup(db)

			_ = db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT job_id, tag FROM job_tags`)
			Expect(err).NotTo(HaveOccurred())

			jobTags := []JobTag{}

			for rows.Next() {
				jobTag := JobTag{}

				err := rows.Scan(&jobTag.JobId, &jobTag.Tag)
				Expect(err).NotTo(HaveOccurred())

				jobTags = append(jobTags, jobTag)
			}

			_ = db.Close()

			Expect(len(jobTags)).To(Equal(5))
			Expect(jobTags[0].JobId).To(Equal(1))
			Expect(jobTags[0].Tag).To(Equal("group1"))
			Expect(jobTags[1].JobId).To(Equal(2))
			Expect(jobTags[1].Tag).To(Equal("group1"))
			Expect(jobTags[2].JobId).To(Equal(2))
			Expect(jobTags[2].Tag).To(Equal("group2"))
			Expect(jobTags[3].JobId).To(Equal(3))
			Expect(jobTags[3].Tag).To(Equal("group2"))
			Expect(jobTags[4].JobId).To(Equal(4))
			Expect(jobTags[4].Tag).To(Equal("group2"))
		})
	})
})

func countRows(db *sql.DB, table string) (int, error) {
	var count int

	err := db.QueryRow("SELECT COUNT(1) FROM " + table).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func setup(db *sql.DB) {
	_, err := db.Exec(`
			INSERT INTO teams(id, name) VALUES
			(1, 'some-team')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
			INSERT INTO pipelines(id, team_id, name, groups) VALUES
			(1, 1, 'pipeline1', '[{"name":"group1","jobs":["job1","job2"]},{"name":"group2","jobs":["job2","job3"]}]'),
			(2, 1, 'pipeline2', '[{"name":"group2","jobs":["job4"]}]')
			`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`
				INSERT INTO jobs(id, pipeline_id, name, config) VALUES
					(1, 1, 'job1', '{"name":"job1"}'),
					(2, 1, 'job2', '{"name":"job2"}'),
					(3, 1, 'job3', '{"name":"job3"}'),
					(4, 2, 'job4', '{"name":"job4"}')
			`)
	Expect(err).NotTo(HaveOccurred())
}
