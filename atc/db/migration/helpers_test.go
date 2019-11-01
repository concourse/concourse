package migration_test

import (
	"database/sql"
	"encoding/json"

	. "github.com/onsi/gomega"
)

func SetupTeamWithBasicAuth(dbConn *sql.DB, team, basicAuth, auth string) {
	_, err := dbConn.Exec("INSERT INTO teams(name, basic_auth, auth) VALUES($1, $2, $3)", team, basicAuth, auth)
	Expect(err).NotTo(HaveOccurred())
}

func SetupTeam(dbConn *sql.DB, team, auth string) {
	_, err := dbConn.Exec("INSERT INTO teams(name, auth) VALUES($1, $2)", team, auth)
	Expect(err).NotTo(HaveOccurred())
}

func SetupPipeline(dbConn *sql.DB, pipeline string) {
	_, err := dbConn.Exec("INSERT INTO pipelines(name, team_id) VALUES($1, 1)", pipeline)
	Expect(err).NotTo(HaveOccurred())
}

func SetupResource(dbConn *sql.DB, resource, config string) {
	_, err := dbConn.Exec("INSERT INTO resources(name, config, pipeline_id) VALUES($1, $2, 1)", resource, config)
	Expect(err).NotTo(HaveOccurred())
}

func ExpectResourceWithType(dbConn *sql.DB, resourceName, resourceType string) {

	Expect(fetchResourceType(dbConn, resourceName)).To(Equal(resourceType))
}

func ExpectTeamWithUsersAndGroups(dbConn *sql.DB, team string, users, groups []string) {

	auth := fetchTeamAuth(dbConn, team)

	Expect(auth["users"]).To(ConsistOf(users))
	Expect(auth["groups"]).To(ConsistOf(groups))
}

func ExpectTeamWithUsersAndGroupsForRole(dbConn *sql.DB, team string, role string, users, groups []string) {

	auth := fetchTeamAuth(dbConn, team)

	authForRole := auth[role].(map[string]interface{})
	Expect(authForRole["users"]).To(ConsistOf(users))
	Expect(authForRole["groups"]).To(ConsistOf(groups))
}

func ExpectTeamWithGithubProvider(dbConn *sql.DB, team, clientId, clientSecret string) {

	auth := fetchTeamAuth(dbConn, team)

	provider, _ := auth["github"].(map[string]interface{})
	Expect(provider["client_id"]).To(Equal(clientId))
	Expect(provider["client_secret"]).To(Equal(clientSecret))
}

func ExpectTeamWithNoAuthProvider(dbConn *sql.DB, team string, noauth bool) {

	auth := fetchTeamAuth(dbConn, team)

	provider, _ := auth["noauth"].(map[string]interface{})
	Expect(provider["noauth"]).To(Equal(noauth))
}

func ExpectTeamWithBasicAuthProvider(dbConn *sql.DB, team, username, password string) {

	auth := fetchTeamAuth(dbConn, team)

	provider, _ := auth["basicauth"].(map[string]interface{})
	Expect(provider["username"]).To(Equal(username))
	Expect(provider["password"]).To(Equal(password))
}

func ExpectTeamWithoutNoAuthProvider(dbConn *sql.DB, team string) {

	auth := fetchTeamAuth(dbConn, team)

	provider, _ := auth["noauth"].(map[string]interface{})
	Expect(provider).To(BeNil())
}

func ExpectTeamWithoutBasicAuthProvider(dbConn *sql.DB, team string) {

	auth := fetchTeamAuth(dbConn, team)

	provider, _ := auth["basicauth"].(map[string]interface{})
	Expect(provider).To(BeNil())
}

func ExpectTeamWithBasicAuth(dbConn *sql.DB, team, username, password string) {

	basicAuth := fetchTeamBasicAuth(dbConn, team)

	Expect(basicAuth["basic_auth_username"]).To(Equal(username))
	Expect(basicAuth["basic_auth_password"]).To(Equal(password))
}

func ExpectTeamWithAuth(dbConn *sql.DB, team, expectedConfig string) {
	auth := readTeamAuth(dbConn, team)
	Expect(auth).To(MatchJSON(expectedConfig))
}

func ExpectTeamWithLegacyAuth(dbConn *sql.DB, team, expectedConfig string) {
	auth := readTeamLegacyAuth(dbConn, team)
	Expect(auth).To(MatchJSON(expectedConfig))
}

func readTeamBasicAuth(dbConn *sql.DB, team string) []byte {
	var auth []byte
	err := dbConn.QueryRow("SELECT basic_auth FROM teams WHERE name = $1", team).Scan(&auth)
	Expect(err).NotTo(HaveOccurred())
	return auth
}

func fetchTeamBasicAuth(dbConn *sql.DB, team string) map[string]string {
	basicAuth := readTeamBasicAuth(dbConn, team)
	var data map[string]string
	err := json.Unmarshal(basicAuth, &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}

func readTeamAuth(dbConn *sql.DB, team string) []byte {
	var auth []byte
	err := dbConn.QueryRow("SELECT auth FROM teams WHERE name = $1", team).Scan(&auth)
	Expect(err).NotTo(HaveOccurred())
	return auth
}

func fetchTeamAuth(dbConn *sql.DB, team string) map[string]interface{} {
	auth := readTeamAuth(dbConn, team)
	var data map[string]interface{}
	err := json.Unmarshal(auth, &data)
	Expect(err).NotTo(HaveOccurred())
	return data
}

func readTeamLegacyAuth(dbConn *sql.DB, team string) []byte {
	var auth []byte
	err := dbConn.QueryRow("SELECT legacy_auth FROM teams WHERE name = $1", team).Scan(&auth)
	Expect(err).NotTo(HaveOccurred())
	return auth
}

func fetchResourceType(dbConn *sql.DB, name string) string {
	var raw string
	err := dbConn.QueryRow("SELECT type FROM resources WHERE name = $1", name).Scan(&raw)
	Expect(err).NotTo(HaveOccurred())
	return raw
}
