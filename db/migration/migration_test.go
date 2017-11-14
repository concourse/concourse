package migration_test

import (
	"database/sql"
	"io/ioutil"
	"strings"

	"github.com/concourse/atc/db/migration"
	"github.com/mattes/migrate/database/postgres"
	"github.com/mattes/migrate/source/go-bindata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Migration", func() {
	var db *sql.DB
	var err error

	const initialSchemaVersion = 1510262030

	BeforeEach(func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = db.Close()
	})

	It("Fails if trying to upgrade prior to migration_version 189", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 188)

		_, err = migration.Open("postgres", postgresRunner.DataSourceName())

		Expect(err.Error()).To(Equal("Cannot upgrade from concourse version < 3.6.0 (db version: 189), current db version: 188"))

		_, err = db.Exec("SELECT version FROM migration_version")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Forces mattes/migrate to a known first version if migration_version is 189", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 189)

		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"})
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})

	It("Runs mattes/migrate if migration_version table does not exist", func() {
		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"})
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})

	It("Doesn't fail if there are no migrations to run", func() {
		SetupSchemaMigrationsTableToExistAtVersion(db, initialSchemaVersion)

		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"})
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})
})

func OpenConnectionWithMigrationFiles(db *sql.DB, files []string) (*sql.DB, error) {
	d, err := postgres.WithInstance(db, &postgres.Config{})
	Expect(err).NotTo(HaveOccurred())

	s, err := bindata.WithInstance(bindata.Resource(
		[]string{"1510262030_initial_schema.up.sql"},
		func(name string) ([]byte, error) {
			return migration.Asset(name)
		}),
	)

	return migration.OpenWithMigrateDrivers(db, "go-bindata", s, "postgres", d)
}

func SetupMigrationVersionTableToExistAtVersion(db *sql.DB, version int) {
	_, err := db.Exec(`CREATE TABLE migration_version(version int)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO migration_version(version) VALUES($1)`, version)
	Expect(err).NotTo(HaveOccurred())
}

func SetupSchemaMigrationsTableToExistAtVersion(db *sql.DB, version int) {
	_, err := db.Exec(`CREATE TABLE schema_migrations(version bigint, dirty boolean)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO schema_migrations(version, dirty) VALUES($1, false)`, version)
	Expect(err).NotTo(HaveOccurred())
}

func SetupSchemaFromFile(db *sql.DB, path string) {
	migrations, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	for _, migration := range strings.Split(string(migrations), ";") {
		_, err = db.Exec(migration)
		Expect(err).NotTo(HaveOccurred())
	}
}

func ExpectSchemaMigrationsTableToHaveVersion(dbConn *sql.DB, expectedVersion int) {
	var dbVersion int
	err := dbConn.QueryRow("SELECT version FROM schema_migrations LIMIT 1").Scan(&dbVersion)
	Expect(err).NotTo(HaveOccurred())
	Expect(dbVersion).To(Equal(expectedVersion))
}

func ExpectMigrationVersionTableNotToExist(dbConn *sql.DB) {
	var exists string
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables where table_name = 'migration_version')").Scan(&exists)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(Equal("false"))
}

func ExpectToBeAbleToInsertData(dbConn *sql.DB) {
	_, err := dbConn.Exec("INSERT INTO teams(id, name) VALUES (10, 'test-team')")
	Expect(err).NotTo(HaveOccurred())

	_, err = dbConn.Exec("INSERT INTO pipelines(id, team_id, name) VALUES (100, 10, 'test-pipeline')")
	Expect(err).NotTo(HaveOccurred())

	_, err = dbConn.Exec("INSERT INTO jobs(id, pipeline_id, name, config) VALUES (1000, 100, 'test-job', '{}')")
	Expect(err).NotTo(HaveOccurred())
}
