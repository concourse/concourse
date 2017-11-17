package migration_test

import (
	"database/sql"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migration"
	"github.com/mattes/migrate/source/go-bindata"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const initialSchemaVersion = 1510262030

var _ = Describe("Migration", func() {
	var db *sql.DB
	var lockDB *sql.DB
	var err error
	var lockFactory lock.LockFactory

	BeforeEach(func() {
		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockFactory = lock.NewLockFactory(lockDB)
	})

	AfterEach(func() {
		_ = db.Close()
		_ = lockDB.Close()
	})

	It("Fails if trying to upgrade from a migration_version < 189", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 188)

		_, err = migration.Open("postgres", postgresRunner.DataSourceName())

		Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 188"))

		_, err = db.Exec("SELECT version FROM migration_version")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Fails if trying to upgrade from a migration_version > 189", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 190)

		_, err = migration.Open("postgres", postgresRunner.DataSourceName())

		Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 190"))

		_, err = db.Exec("SELECT version FROM migration_version")
		Expect(err).NotTo(HaveOccurred())
	})

	It("Forces mattes/migrate to a known first version if migration_version is 189", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 189)

		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"}, lockFactory)
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})

	It("Runs mattes/migrate if migration_version table does not exist", func() {
		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"}, lockFactory)
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})

	It("Doesn't fail if there are no migrations to run", func() {
		SetupSchemaMigrationsTableToExistAtVersion(db, initialSchemaVersion)

		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

		dbConn, err := OpenConnectionWithMigrationFiles(db, []string{"1510262030_initial_schema.up.sql"}, lockFactory)
		Expect(err).NotTo(HaveOccurred())
		defer dbConn.Close()

		ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

		ExpectMigrationVersionTableNotToExist(dbConn)

		ExpectToBeAbleToInsertData(dbConn)
	})

	It("Locks the database so multiple ATCs don't all run migrations at the same time", func() {
		SetupMigrationVersionTableToExistAtVersion(db, 189)

		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

		var wg sync.WaitGroup
		wg.Add(3)

		go TryRunMigrationsAndVerifyResult([]string{"1510262030_initial_schema.up.sql"}, lockFactory, &wg)
		go TryRunMigrationsAndVerifyResult([]string{"1510262030_initial_schema.up.sql"}, lockFactory, &wg)
		go TryRunMigrationsAndVerifyResult([]string{"1510262030_initial_schema.up.sql"}, lockFactory, &wg)

		wg.Wait()
	})
})

func TryRunMigrationsAndVerifyResult(files []string, lockFactory lock.LockFactory, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()

	db, err := sql.Open("postgres", postgresRunner.DataSourceName())
	Expect(err).NotTo(HaveOccurred())
	defer db.Close()

	dbConn, err := OpenConnectionWithMigrationFiles(db, files, lockFactory)
	Expect(err).NotTo(HaveOccurred())
	defer dbConn.Close()

	ExpectSchemaMigrationsTableToHaveVersion(dbConn, initialSchemaVersion)

	ExpectMigrationVersionTableNotToExist(dbConn)

	ExpectToBeAbleToInsertData(dbConn)
}

func OpenConnectionWithMigrationFiles(db *sql.DB, files []string, lockFactory lock.LockFactory) (*sql.DB, error) {
	s, _ := bindata.WithInstance(bindata.Resource(
		[]string{"1510262030_initial_schema.up.sql"},
		func(name string) ([]byte, error) {
			return migration.Asset(name)
		}),
	)

	return migration.OpenWithMigrateDrivers(db, "go-bindata", s, lockFactory)
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
	rand.Seed(time.Now().UnixNano())

	teamID := rand.Intn(10000)
	_, err := dbConn.Exec("INSERT INTO teams(id, name) VALUES ($1, $2)", teamID, strconv.Itoa(teamID))
	Expect(err).NotTo(HaveOccurred())

	pipelineID := rand.Intn(10000)
	_, err = dbConn.Exec("INSERT INTO pipelines(id, team_id, name) VALUES ($1, $2, $3)", pipelineID, teamID, strconv.Itoa(pipelineID))
	Expect(err).NotTo(HaveOccurred())

	jobID := rand.Intn(10000)
	_, err = dbConn.Exec("INSERT INTO jobs(id, pipeline_id, name, config) VALUES ($1, $2, $3, '{}')", jobID, pipelineID, strconv.Itoa(jobID))
	Expect(err).NotTo(HaveOccurred())
}
