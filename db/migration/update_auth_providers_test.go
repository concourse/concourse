package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Update auth providers", func() {
	const preMigrationVersion = 1513895878
	const postMigrationVersion = 1516643303

	var (
		db *sql.DB
	)

	Context("Down", func() {
		It("migrates basic auth data to separate field", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"basicauth": {"username": "username", "password": "password"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithBasicAuth(db, "main", "username", "password")
			db.Close()
		})

		It("does not modify existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithGithubProvider(db, "main", "some-client-id", "some-client-secret")
			db.Close()
		})

		It("removes the basicauth provider from providers list", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"basicauth": {"username": "username", "password": "password"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main")
			db.Close()
		})

		It("removes the noauth provider from providers list", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "main", `{"noauth": {"noauth": true}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})
	})

	Context("Up", func() {
		It("migrates basic auth data to empty providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			db.Close()
		})

		It("migrates basic auth data to null providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, `null`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			db.Close()
		})

		It("merges basic auth data with existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			ExpectTeamWithGithubProvider(db, "main", "some-client-id", "some-client-secret")
			db.Close()
		})

		It("does not migrate malformed basic auth data", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{"u": "username", "p": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main")
			db.Close()
		})

		It("does not migrate empty json basic auth data", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main-empty", `{}`, ``)
			SetupTeamWithBasicAuth(db, "main-null", `null`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main-empty")
			ExpectTeamWithoutBasicAuthProvider(db, "main-null")
			db.Close()
		})

		It("does not add noauth provider when basic auth is configured", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})

		It("does not add noauth provider when there are existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main", `{}`, `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})

		It("adds noauth provider when no other auth methods are configured", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamWithBasicAuth(db, "main-empty-blank", `{}`, ``)
			SetupTeamWithBasicAuth(db, "main-null-blank", `null`, ``)
			SetupTeamWithBasicAuth(db, "main-empty-empty", `{}`, `{}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithNoAuthProvider(db, "main-empty-blank", true)
			ExpectTeamWithNoAuthProvider(db, "main-null-blank", true)
			ExpectTeamWithNoAuthProvider(db, "main-empty-empty", true)
			db.Close()
		})
	})
})
