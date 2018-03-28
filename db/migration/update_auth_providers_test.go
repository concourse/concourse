package migration_test

import (
	"database/sql"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			SetupTeamForDownMigration(db, "main", `{"basicauth": {"username": "username", "password": "password"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithBasicAuth(db, "main", "username", "password")
			db.Close()
		})

		It("does not modify existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeamForDownMigration(db, "main", `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithGithubProvider(db, "main", "some-client-id", "some-client-secret")
			db.Close()
		})

		It("removes the basicauth provider from providers list", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeamForDownMigration(db, "main", `{"basicauth": {"username": "username", "password": "password"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main")
			db.Close()
		})

		It("removes the noauth provider from providers list", func() {

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeamForDownMigration(db, "main", `{"noauth": {"noauth": true}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})
	})

	Context("Up", func() {
		It("migrates basic auth data to empty providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			db.Close()
		})

		It("migrates basic auth data to null providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, `null`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			db.Close()
		})

		It("merges basic auth data with existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithBasicAuthProvider(db, "main", "username", "password")
			ExpectTeamWithGithubProvider(db, "main", "some-client-id", "some-client-secret")
			db.Close()
		})

		It("does not migrate malformed basic auth data", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{"u": "username", "p": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main")
			db.Close()
		})

		It("does not migrate empty json basic auth data", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main-empty", `{}`, ``)
			SetupTeamForUpMigration(db, "main-null", `null`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutBasicAuthProvider(db, "main-empty")
			ExpectTeamWithoutBasicAuthProvider(db, "main-null")
			db.Close()
		})

		It("does not add noauth provider when basic auth is configured", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{"basic_auth_username": "username", "basic_auth_password": "password"}`, ``)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})

		It("does not add noauth provider when there are existing providers", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main", `{}`, `{"github": {"client_id": "some-client-id", "client_secret": "some-client-secret"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithoutNoAuthProvider(db, "main")
			db.Close()
		})

		It("adds noauth provider when no other auth methods are configured", func() {

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeamForUpMigration(db, "main-empty-blank", `{}`, ``)
			SetupTeamForUpMigration(db, "main-null-blank", `null`, ``)
			SetupTeamForUpMigration(db, "main-empty-empty", `{}`, `{}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithNoAuthProvider(db, "main-empty-blank", true)
			ExpectTeamWithNoAuthProvider(db, "main-null-blank", true)
			ExpectTeamWithNoAuthProvider(db, "main-empty-empty", true)
			db.Close()
		})
	})
})

func SetupTeamForUpMigration(dbConn *sql.DB, team, basicAuth, auth string) {
	_, err := dbConn.Exec("INSERT INTO teams(name, basic_auth, auth) VALUES($1, $2, $3)", team, basicAuth, auth)
	Expect(err).NotTo(HaveOccurred())
}

func SetupTeamForDownMigration(dbConn *sql.DB, team, auth string) {
	_, err := dbConn.Exec("INSERT INTO teams(name, auth) VALUES($1, $2)", team, auth)
	Expect(err).NotTo(HaveOccurred())
}

func ExpectTeamWithGithubProvider(dbConn *sql.DB, team, clientId, clientSecret string) {

	providers := fetchTeamAuthProviders(dbConn, team)

	provider, _ := providers["github"].(map[string]interface{})
	Expect(provider["client_id"]).To(Equal(clientId))
	Expect(provider["client_secret"]).To(Equal(clientSecret))
}

func ExpectTeamWithNoAuthProvider(dbConn *sql.DB, team string, noauth bool) {

	providers := fetchTeamAuthProviders(dbConn, team)

	provider, _ := providers["noauth"].(map[string]interface{})
	Expect(provider["noauth"]).To(Equal(noauth))
}

func ExpectTeamWithBasicAuthProvider(dbConn *sql.DB, team, username, password string) {

	providers := fetchTeamAuthProviders(dbConn, team)

	provider, _ := providers["basicauth"].(map[string]interface{})
	Expect(provider["username"]).To(Equal(username))
	Expect(provider["password"]).To(Equal(password))
}

func ExpectTeamWithoutNoAuthProvider(dbConn *sql.DB, team string) {

	providers := fetchTeamAuthProviders(dbConn, team)

	provider, _ := providers["noauth"].(map[string]interface{})
	Expect(provider).To(BeNil())
}

func ExpectTeamWithoutBasicAuthProvider(dbConn *sql.DB, team string) {

	providers := fetchTeamAuthProviders(dbConn, team)

	provider, _ := providers["basicauth"].(map[string]interface{})
	Expect(provider).To(BeNil())
}

func ExpectTeamWithBasicAuth(dbConn *sql.DB, team, username, password string) {

	basicAuth := fetchTeamBasicAuth(dbConn, team)

	Expect(basicAuth["basic_auth_username"]).To(Equal(username))
	Expect(basicAuth["basic_auth_password"]).To(Equal(password))
}

func fetchTeamBasicAuth(dbConn *sql.DB, team string) map[string]string {

	var basicAuth []byte
	err := dbConn.QueryRow("SELECT basic_auth FROM teams WHERE name = $1", team).Scan(&basicAuth)
	Expect(err).NotTo(HaveOccurred())

	var data map[string]string
	err = json.Unmarshal(basicAuth, &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}

func fetchTeamAuthProviders(dbConn *sql.DB, team string) map[string]interface{} {

	var providers []byte
	err := dbConn.QueryRow("SELECT auth FROM teams WHERE name = $1", team).Scan(&providers)
	Expect(err).NotTo(HaveOccurred())

	var data map[string]interface{}
	err = json.Unmarshal(providers, &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}
