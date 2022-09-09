package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add secondary ordering column", func() {
	const preMigrationVersion = 1619180097
	const postMigrationVersion = 1619180098

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates populates secondary ordering", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

			_, err := db.Exec(`
					INSERT INTO teams(id, name) VALUES
					(1, 'team1'),
					(2, 'team2')
					`)
			Expect(err).NotTo(HaveOccurred())

			_, err = db.Exec(`
					INSERT INTO pipelines(id, team_id, name, instance_vars) VALUES
					(1, 1, 'group1', '{"version": 1}'::jsonb),
					(2, 1, 'group1', '{"version": 2}'::jsonb),
					(3, 1, 'group2', '{"version": 1}'::jsonb),
					(4, 1, 'group1', NULL),
					(5, 1, 'pipeline', NULL),
					(6, 2, 'group1', '{"version": 3}'::jsonb)
					`)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			rows, err := db.Query(`SELECT id, secondary_ordering FROM pipelines ORDER BY id ASC`)
			Expect(err).NotTo(HaveOccurred())

			type pipelineOrdering struct {
				pipelineID        int
				secondaryOrdering int
			}

			ordering := []pipelineOrdering{}
			for rows.Next() {
				var o pipelineOrdering

				err := rows.Scan(&o.pipelineID, &o.secondaryOrdering)
				Expect(err).NotTo(HaveOccurred())

				ordering = append(ordering, o)
			}

			_ = db.Close()

			Expect(ordering).To(Equal([]pipelineOrdering{
				{1, 1},
				{2, 2},
				{3, 1},
				{4, 3},
				{5, 1},
				{6, 1},
			}))
		})
	})
})
