package migration_test

import (
	"database/sql"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fix Build Private Plan", func() {
	const preMigrationVersion = 1551384519
	const postMigrationVersion = 1551384520

	var (
		db *sql.DB
	)

	Context("Up", func() {
		It("ignores NULL plans", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuild(db, "some-build")
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectBuildWithNullPlan(db, "some-build")
			db.Close()
		})

		It("removes the 'plan' key", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuildWithPlan(db, "some-build", `{"plan":{"some":"plan"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectBuildWithPlan(db, "some-build", `{"some":"plan"}`)
			db.Close()
		})

		It("updates multiple plans", func() {
			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuildWithPlan(db, "some-build", `{"plan":{"some":"plan"}}`)
			SetupBuildWithPlan(db, "some-other-build", `{"plan":{"some":"other-plan"}}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			ExpectBuildWithPlan(db, "some-build", `{"some":"plan"}`)
			ExpectBuildWithPlan(db, "some-other-build", `{"some":"other-plan"}`)
			db.Close()
		})
	})

	Context("Down", func() {
		It("ignores NULL plans", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuild(db, "some-build")
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectBuildWithNullPlan(db, "some-build")
			db.Close()
		})

		It("nests the plan under a 'plan' key", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuildWithPlan(db, "some-build", `{"some":"plan"}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectBuildWithPlan(db, "some-build", `{"plan":{"some":"plan"}}`)
			db.Close()
		})

		It("updates multiple plans", func() {
			db = postgresRunner.OpenDBAtVersion(postMigrationVersion)
			SetupTeam(db, "some-team", "{}")
			SetupBuildWithPlan(db, "some-build", `{"some":"plan"}`)
			SetupBuildWithPlan(db, "some-other-build", `{"some":"other-plan"}`)
			db.Close()

			db = postgresRunner.OpenDBAtVersion(preMigrationVersion)
			ExpectBuildWithPlan(db, "some-build", `{"plan":{"some":"plan"}}`)
			ExpectBuildWithPlan(db, "some-other-build", `{"plan":{"some":"other-plan"}}`)
			db.Close()
		})
	})
})

func SetupBuild(dbConn *sql.DB, name string) {
	_, err := dbConn.Exec("INSERT INTO builds(name, status, team_id) VALUES($1, 'started', 1)", name)
	Expect(err).NotTo(HaveOccurred())
}

func SetupBuildWithPlan(dbConn *sql.DB, name, plan string) {
	_, err := dbConn.Exec("INSERT INTO builds(name, status, team_id, private_plan) VALUES($1, 'started', 1, $2)", name, plan)
	Expect(err).NotTo(HaveOccurred())
}

func ExpectBuildWithNullPlan(dbConn *sql.DB, name string) {

	plan := fetchBuildPlan(dbConn, name)

	Expect(plan.Valid).To(BeFalse())
}

func ExpectBuildWithPlan(dbConn *sql.DB, name, expectedPlan string) {

	plan := fetchBuildPlan(dbConn, name)

	Expect(plan.Valid).To(BeTrue())
	Expect(plan.String).To(Equal(expectedPlan))
}

func fetchBuildPlan(dbConn *sql.DB, name string) sql.NullString {
	var plan sql.NullString
	err := dbConn.QueryRow("SELECT private_plan FROM builds WHERE name = $1", name).Scan(&plan)
	Expect(err).NotTo(HaveOccurred())
	return plan
}
