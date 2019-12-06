package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Update cf auth", func() {
	const preMigrationVersion = 1569945021
	const postMigrationVersion = 1572899256

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("migrates cf auth with form ORG:SPACE to ORG:SPACE:developer", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", `{"owner":{"groups":["cf:test-org:space1","cf:test-org"],"users":["local:test"]}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithAuth(db, "main", `{"owner":{"groups":["cf:test-org:space1:developer","cf:test-org"],"users":["local:test"]}}`)
			db.Close()
		})

		It("migrates cf auth with a bunch of different cf:cf combinations", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", `{"owner":{"groups":["cf","cf:cf","cf:cf:cf","cf:cf:cf:cf","cf:cf:cf:cf:cf"],"users":["local:cf"]}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithAuth(db, "main", `{"owner":{"groups":["cf","cf:cf","cf:cf:cf:developer","cf:cf:cf:cf:developer","cf:cf:cf:cf:cf:developer"],"users":["local:cf"]}}`)
			db.Close()
		})

		It("does not migrate any pivotal-cf github orgs", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", `{"owner":{"groups":["github:pivotal-cf","github:pivotal-cf:cf","github:pivotal-cf:cf:cf","github:pivotal-cf:cf:cf:cf:cf","github:pivotal-cf:cf:cf:cf:cf:cf"],"users":["local:cf"]}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithAuth(db, "main", `{"owner":{"groups":["github:pivotal-cf","github:pivotal-cf:cf","github:pivotal-cf:cf:cf","github:pivotal-cf:cf:cf:cf:cf","github:pivotal-cf:cf:cf:cf:cf:cf"],"users":["local:cf"]}}`)
			db.Close()
		})
	})

	//TODO should we remove ORG:SPACE:auditor and ORG:SPACE:manager
	Context("Down", func() {
		It("migrate cf auth with form ORG:SPACE:developer to ORG:SPACE", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"owner":{"groups":["cf:test-org:space1:developer","cf:test-org"],"users":["local:test"]}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithAuth(db, "main", `{"owner":{"groups":["cf:test-org:space1","cf:test-org"],"users":["local:test"]}}`)
			db.Close()
		})

		It("does not migrate auditor and manager roles", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"owner":{"groups":["cf:test-org:space1:auditor","cf:test-org:space1:manager"],"users":["local:test"]}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithAuth(db, "main", `{"owner":{"groups":["cf:test-org:space1:auditor","cf:test-org:space1:manager"],"users":["local:test"]}}`)
			db.Close()
		})
	})
})
