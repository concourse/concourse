package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Add team roles", func() {

	const preMigrationVersion = 1533934775
	const postMigrationVersion = 1537196857

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("successfully adds the default 'owner' role to existing team auth", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", `{"users": ["local:user1"], "groups": [] }`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithUsersAndGroupsForRole(db, "main", "owner", []string{"local:user1"}, []string{})
			db.Close()

		})
	})

	Context("Down", func() {
		It("successfully removes roles from team auth", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{ "owner": {"users": ["local:user1"], "groups": [] }}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithUsersAndGroups(db, "main", []string{"local:user1"}, []string{})
			db.Close()

		})
	})

})
