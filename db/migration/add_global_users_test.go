package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add global users", func() {
	const preMigrationVersion = 1528314953
	const postMigrationVersion = 1528470872

	var (
		db *sql.DB
	)

	Context("Up", func() {

		testMigration := func(oldConfig string, newConfig string) {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", oldConfig)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectTeamWithAuth(db, "main", newConfig)
			ExpectTeamWithLegacyAuth(db, "main", oldConfig)
			db.Close()
		}

		It("migrates github data to users/groups format", func() {
			legacyConfig := `
			{
				"github": {
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"organizations": ["some-other-org"],
					"teams": [{
						"organization_name": "some-org",
						"team_name": "some-team"
					}],
					"users": ["some-user"]
				}
			}
			`
			newConfig := `
			{
				"users": ["github:some-user"],
				"groups": ["github:some-org:some-team", "github:some-other-org"]
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("migrates basic auth data to users/groups format", func() {
			legacyConfig := `
			{
				"basicauth": {
					"username": "some-user",
					"password": "some-password"
				}
			}
			`
			newConfig := `
			{
				"users": ["local:some-user"],
				"groups": []
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("migrates uaa data to users/groups format", func() {
			legacyConfig := `
			{
				"uaa": {
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"auth_url": "https://example.com/auth",
					"token_url": "https://example.com/token",
					"cf_spaces": ["some-space-guid"],
					"cf_url": "https://example.com/api"
				}
			}
			`
			newConfig := `
			{
				"users": [],
				"groups": ["cf:some-space-guid"]
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("migrates gitlab data to users/groups format", func() {
			legacyConfig := `
			{
				"gitlab": {
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"groups": ["some-group"],
					"auth_url": "https://example.com/auth",
					"token_url": "https://example.com/token",
					"api_url": "https://example.com/api"
				}
			}
			`
			newConfig := `
			{
				"users": [],
				"groups": ["gitlab:some-group"]
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("migrates oauth data to users/groups format", func() {
			legacyConfig := `
			{
				"oauth": {
					"display_name": "provider",
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"auth_url": "https://example.com/auth",
					"token_url": "https://example.com/token",
					"auth_url_params": {
						"some-param": "some-value"
					},
					"scope": "some-scope"
				}
			}
			`
			newConfig := `
			{
				"users": [],
				"groups": ["oauth:some-scope"]
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("migrates oidc data to users/groups format", func() {
			legacyConfig := `
			{
				"oauth_oidc": {
					"display_name": "provider",
					"client_id": "some-client",
					"client_secret": "some-secret",
					"user_id": ["some-user"],
					"groups": ["some-group"],
					"custom_groups_name": "some-groups-key",
					"auth_url": "https://example.com/auth",
					"token_url": "https://example.com/token",
					"auth_url_params": {
						"some-param": "some-value"
					},
					"scope": "some-scope"
				}
			}
			`
			newConfig := `
			{
				"users": ["oidc:some-user"],
				"groups": ["oidc:some-group"]
			}
			`
			testMigration(legacyConfig, newConfig)
		})

		It("fails to migrate if bitbucket cloud is present", func() {
			legacyConfig := `
			{
				"bitbucket-cloud": {
					"client_id": "some-client",
					"client_secret": "some-client-secret",
					"users": ["some-user"],
					"teams": [{
						"team_name": "some-team",
						"role": "member"
					}],
					"repositories": [{
						"owner_name": "some-owner",
						"repository_name": "some-repository"
					}],
					"auth_url": "https://example.com/auth",
					"token_url": "https://example.com/token",
					"apiurl": "https://example.com/api"
				}
			}
			`
			db := postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", legacyConfig)
			db.Close()

			db, err := postgresRunner.TryOpenDBAtVersion(postMigrationVersion)
			Expect(err).To(HaveOccurred())
		})

		It("fails to migrate if bitbucket server is present", func() {
			legacyConfig := `
			{
				"bitbucket-server": {
					"consumer_key": "/tmp/concourse-dev/keys/web/session_signing_key",
					"private_key": {
						"N": 0,
						"E": 0,
						"D": 0,
						"Primes": [0, 0],
						"Precomputed": {
							"Dp": 0,
							"Dq": 0,
							"Qinv": 0,
							"CRTValues": []
						}
					},
					"endpoint": "https://example.com/endpoint",
					"users": ["some-user"],
					"projects": ["some-project"],
					"repositories": [{
						"owner_name": "some-owner",
						"repository_name": "some-repository"
					}]
				}
			}
			`
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", legacyConfig)
			db.Close()

			_, err := postgresRunner.TryOpenDBAtVersion(postMigrationVersion)
			Expect(err).To(HaveOccurred())
		})

		It("fails to migrate uaa if teams are using different providers of the same type", func() {
			legacyConfigMain := `
			{
				"uaa": {
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"auth_url": "https://main.com/auth",
					"token_url": "https://main.com/token",
					"cf_spaces": ["some-space-guid"],
					"cf_url": "https://main.com/api"
				}
			}
			`
			legacyConfigOther := `
			{
				"uaa": {
					"client_id": "some-client-id",
					"client_secret": "some-client-secret",
					"auth_url": "https://other.com/auth",
					"token_url": "https://other.com/token",
					"cf_spaces": ["some-space-guid"],
					"cf_url": "https://other.com/api"
				}
			}
			`

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", legacyConfigMain)
			SetupTeam(db, "other", legacyConfigOther)
			db.Close()

			_, err := postgresRunner.TryOpenDBAtVersion(postMigrationVersion)
			Expect(err).To(HaveOccurred())
		})

		It("fails to migrate if two teams have the same basic auth username", func() {
			legacyConfigMain := `
			{
				"basicauth": {
					"username": "some-user",
					"password": "some-password"
				}
			}
			`
			legacyConfigOther := `
			{
				"basicauth": {
					"username": "some-user",
					"password": "another-password"
				}
			}
			`

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "main", legacyConfigMain)
			SetupTeam(db, "other", legacyConfigOther)
			db.Close()

			_, err := postgresRunner.TryOpenDBAtVersion(postMigrationVersion)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Down", func() {
		It("works when only main team has changed auth", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)

			_, err := db.Exec("INSERT INTO teams(name, legacy_auth) VALUES('main', NULL)")
			Expect(err).NotTo(HaveOccurred())
			_, err = db.Exec(`INSERT INTO teams(name, legacy_auth) VALUES('another-team', '{"some-legacy-config": true}')`)
			Expect(err).NotTo(HaveOccurred())

			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectTeamWithAuth(db, "another-team", `{"some-legacy-config": true}`)
			db.Close()
		})

		It("fails when non-main teams have changed auth", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			_, err := db.Exec("INSERT INTO teams(name, legacy_auth) VALUES('some-team', NULL)")
			Expect(err).NotTo(HaveOccurred())
			db.Close()

			_, err = postgresRunner.TryOpenDBAtVersion(preMigrationVersion)
			Expect(err).To(HaveOccurred())
		})
	})
})
