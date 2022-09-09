package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Extract resource type", func() {
	const preMigrationVersion = 1557237784
	const postMigrationVersion = 1561558376

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("handles resource with empty config", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", `{}`)
			SetupPipeline(db, "some-pipeline")
			SetupResource(db, "some-resource", `{}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectResourceWithType(db, "some-resource", "")
			db.Close()
		})
	})
})
