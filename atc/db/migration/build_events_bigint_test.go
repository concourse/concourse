package migration_test

import (
	"database/sql"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("build_events bigint indexes", func() {
	const preMigrationVersion = 1602860421
	const postMigrationVersion = 1606068653
	const downMigrationVersion = 1603405319

	var db *sql.DB
	var teamID int
	var pipelineID int

	explain := func(query string, params ...interface{}) string {
		rows, err := db.Query("SET enable_seqscan = OFF; EXPLAIN "+query, params...)
		Expect(err).ToNot(HaveOccurred())

		lines := []string{}
		for rows.Next() {
			var line string
			Expect(rows.Scan(&line)).To(Succeed())
			lines = append(lines, line)
		}

		return strings.Join(lines, "\n")
	}

	BeforeEach(func() {
		db = postgresRunner.OpenDBAtVersion(preMigrationVersion)

		err := db.QueryRow(`
			INSERT INTO teams (name, auth)
			VALUES ('some-team', '{}')
			RETURNING id
		`).Scan(&teamID)
		Expect(err).ToNot(HaveOccurred())

		err = db.QueryRow(`
			INSERT INTO pipelines (name, team_id)
			VALUES ('some-pipeline', $1)
			RETURNING id
		`, teamID).Scan(&pipelineID)
		Expect(err).ToNot(HaveOccurred())

		postgresRunner.MigrateToVersion(postMigrationVersion)
	})

	AfterEach(func() {
		Expect(db.Close()).To(Succeed())
	})

	Describe("Up", func() {
		It("has indexes for both build_id_old and build_id in parent table", func() {
			plan := explain(`SELECT * FROM build_events WHERE build_id = 1`)
			Expect(plan).To(ContainSubstring("Index Scan"))

			plan = explain(`SELECT * FROM build_events WHERE build_id_old = 1`)
			Expect(plan).To(ContainSubstring("Index Scan"))
		})

		It("has indexes for both build_id_old and build_id in pipeline partitions", func() {
			plan := explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id = 1`,
				pipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))

			plan = explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id_old = 1`,
				pipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))
		})

		It("has indexes for only build_id in team partitions", func() {
			plan := explain(fmt.Sprintf(
				`SELECT * FROM team_build_events_%d WHERE build_id = 1`,
				teamID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))

			plan = explain(fmt.Sprintf(
				`SELECT * FROM team_build_events_%d WHERE build_id_old = 1`,
				pipelineID,
			))
			Expect(plan).To(ContainSubstring("Seq Scan"))
		})

		It("has indexes for newly created team partitions", func() {
			var newTeamID int
			err := db.QueryRow(`
				INSERT INTO teams (name, auth)
				VALUES ('some-other-team', '{}')
				RETURNING id
			`).Scan(&newTeamID)
			Expect(err).ToNot(HaveOccurred())

			plan := explain(fmt.Sprintf(
				`SELECT * FROM team_build_events_%d WHERE build_id = 1`,
				newTeamID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))
		})

		It("has indexes for both build_id and build_id_old in newly created pipeline partitions", func() {
			var newPipelineID int
			err := db.QueryRow(`
				INSERT INTO pipelines (name, team_id)
				VALUES ('some-other-pipeline', $1)
				RETURNING id
			`, teamID).Scan(&newPipelineID)
			Expect(err).ToNot(HaveOccurred())

			plan := explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id = 1`,
				newPipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))

			plan = explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id_old = 1`,
				newPipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))
		})
	})

	Describe("Down", func() {
		BeforeEach(func() {
			postgresRunner.MigrateToVersion(downMigrationVersion)
		})

		It("has index for build_id", func() {
			plan := explain(`SELECT * FROM build_events WHERE build_id = 1`)
			Expect(plan).To(ContainSubstring("Index Scan"))
			Expect(plan).ToNot(ContainSubstring("build_id_old"))
		})

		It("has index for build_id_old in pipeline partitions", func() {
			plan := explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id_old = 1`,
				pipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))
		})

		It("does not have index for build_id in pipeline partitions", func() {
			plan := explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id = 1`,
				pipelineID,
			))
			Expect(plan).To(ContainSubstring("Seq Scan"))
		})

		It("does not have index for build_id in team partitions", func() {
			plan := explain(fmt.Sprintf(
				`SELECT * FROM team_build_events_%d WHERE build_id = 1`,
				teamID,
			))
			Expect(plan).To(ContainSubstring("Seq Scan"))
		})

		It("does not have index for build_id in newly created team partitions", func() {
			var newTeamID int
			err := db.QueryRow(`
				INSERT INTO teams (name, auth)
				VALUES ('some-other-team', '{}')
				RETURNING id
			`).Scan(&newTeamID)
			Expect(err).ToNot(HaveOccurred())

			plan := explain(fmt.Sprintf(
				`SELECT * FROM team_build_events_%d WHERE build_id = 1`,
				newTeamID,
			))
			Expect(plan).To(ContainSubstring("Seq Scan"))
		})

		It("has index for build_id in newly created pipeline partitions", func() {
			var newPipelineID int
			err := db.QueryRow(`
				INSERT INTO pipelines (name, team_id)
				VALUES ('some-other-pipeline', $1)
				RETURNING id
			`, teamID).Scan(&newPipelineID)
			Expect(err).ToNot(HaveOccurred())

			plan := explain(fmt.Sprintf(
				`SELECT * FROM pipeline_build_events_%d WHERE build_id = 1`,
				newPipelineID,
			))
			Expect(plan).To(ContainSubstring("Index Scan"))
		})
	})
})
