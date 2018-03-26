package migration_test

import (
	"database/sql"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migration"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const initialSchemaVersion = 1510262030
const upgradedSchemaVersion = 1510670987

var _ = Describe("Migration", func() {
	var (
		err         error
		db          *sql.DB
		lockDB      *sql.DB
		lockFactory lock.LockFactory
		strategy    encryption.Strategy
	)

	BeforeEach(func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockFactory = lock.NewLockFactory(lockDB)

		strategy = encryption.NewNoEncryption()
	})

	AfterEach(func() {
		_ = db.Close()
		_ = lockDB.Close()
	})

	Context("Migration test run", func() {
		It("Runs all the migrations", func() {
			migrator := migration.NewMigrator(db, lockFactory, strategy)

			err := migrator.Up()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Version Check", func() {
		It("CurrentVersion reports the current version stored in the database", func() {

			myDatabaseVersion := 1234567890

			SetupSchemaMigrationsTableToExistAtVersion(db, myDatabaseVersion)

			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"2000000000_latest_migration_does_not_matter.up.sql",
			})

			version, err := migrator.CurrentVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(myDatabaseVersion))
		})

		It("SupportedVersion reports the highest supported migration version", func() {

			SetupSchemaMigrationsTableToExistAtVersion(db, initialSchemaVersion)

			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"300000_this_is_to_prove_we_dont_use_string_sort.up.sql",
				"2000000000_latest_migration.up.sql",
			})

			version, err := migrator.SupportedVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(2000000000))
		})

		It("Ignores files it can't parse", func() {

			SetupSchemaMigrationsTableToExistAtVersion(db, initialSchemaVersion)

			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"300000_this_is_to_prove_we_dont_use_string_sort.up.sql",
				"2000000000_latest_migration.up.sql",
				"migrations.go",
			})

			version, err := migrator.SupportedVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(2000000000))
		})
	})

	Context("Upgrade", func() {
		Context("sql migrations", func() {
			It("Fails if trying to upgrade from a migration_version < 189", func() {
				SetupMigrationVersionTableToExistAtVersion(db, 188)

				migrator := migration.NewMigrator(db, lockFactory, strategy)

				err := migrator.Up()
				Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 188"))

				_, err = db.Exec("SELECT version FROM migration_version")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Fails if trying to upgrade from a migration_version > 189", func() {
				SetupMigrationVersionTableToExistAtVersion(db, 190)

				migrator := migration.NewMigrator(db, lockFactory, strategy)

				err := migrator.Up()
				Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 190"))

				_, err = db.Exec("SELECT version FROM migration_version")
				Expect(err).NotTo(HaveOccurred())
			})

			It("Forces mattes/migrate to a known first version if migration_version is 189", func() {
				SetupMigrationVersionTableToExistAtVersion(db, 189)

				SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				ExpectSchemaMigrationsTableToHaveVersion(db, initialSchemaVersion)

				ExpectMigrationVersionTableNotToExist(db)

				ExpectToBeAbleToInsertData(db)
			})

			It("Runs mattes/migrate if migration_version table does not exist", func() {

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				ExpectSchemaMigrationsTableToHaveVersion(db, initialSchemaVersion)

				ExpectMigrationVersionTableNotToExist(db)

				ExpectToBeAbleToInsertData(db)
			})

			It("fails if the migration version is in a dirty state", func() {
				SetupSchemaMigrationsTableToExistAtVersionWithDirtyState(db, 190, true)

				SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				err := migrator.Up()
				Expect(err).To(HaveOccurred())
			})

			It("truncates the table if the migration version is in a dirty state", func() {
				SetupSchemaMigrationsTableToExistAtVersionWithDirtyState(db, 190, true)

				SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				err := migrator.Up()
				Expect(err).To(HaveOccurred())
			})

			It("Doesn't fail if there are no migrations to run", func() {
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				err = migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				ExpectSchemaMigrationsTableToHaveVersion(db, initialSchemaVersion)

				ExpectMigrationVersionTableNotToExist(db)

				ExpectToBeAbleToInsertData(db)
			})

			It("Locks the database so multiple ATCs don't all run migrations at the same time", func() {
				SetupMigrationVersionTableToExistAtVersion(db, 189)

				SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
				})

				var wg sync.WaitGroup
				wg.Add(3)

				go TryRunUpAndVerifyResult(db, migrator, &wg)
				go TryRunUpAndVerifyResult(db, migrator, &wg)
				go TryRunUpAndVerifyResult(db, migrator, &wg)

				wg.Wait()
			})
		})

		Context("golang migrations", func() {
			It("contains the correct migration version", func() {

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
					"1510262030_initial_schema.up.sql",
					"1516643303_update_auth_providers.up.go",
				})

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				ExpectSchemaMigrationsTableToHaveVersion(db, 1516643303)
			})
		})
	})

	Context("Downgrade", func() {

		It("Downgrades to a given version", func() {
			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
			})

			err := migrator.Up()
			Expect(err).NotTo(HaveOccurred())

			ExpectSchemaMigrationsTableToHaveVersion(db, upgradedSchemaVersion)

			err = migrator.Migrate(initialSchemaVersion)
			Expect(err).NotTo(HaveOccurred())

			ExpectSchemaMigrationsTableToHaveVersion(db, initialSchemaVersion)

			ExpectToBeAbleToInsertData(db)
		})

		It("Doesn't fail if already at the requested version", func() {
			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
			})

			err := migrator.Migrate(upgradedSchemaVersion)
			Expect(err).NotTo(HaveOccurred())

			ExpectSchemaMigrationsTableToHaveVersion(db, upgradedSchemaVersion)

			err = migrator.Migrate(upgradedSchemaVersion)
			Expect(err).NotTo(HaveOccurred())

			ExpectSchemaMigrationsTableToHaveVersion(db, upgradedSchemaVersion)

			ExpectToBeAbleToInsertData(db)
		})

		It("Locks the database so multiple consumers don't run downgrade at the same time", func() {
			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, []string{
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
			})

			err := migrator.Up()
			Expect(err).NotTo(HaveOccurred())

			var wg sync.WaitGroup
			wg.Add(3)

			go TryRunMigrateAndVerifyResult(db, migrator, initialSchemaVersion, &wg)
			go TryRunMigrateAndVerifyResult(db, migrator, initialSchemaVersion, &wg)
			go TryRunMigrateAndVerifyResult(db, migrator, initialSchemaVersion, &wg)

			wg.Wait()
		})
	})

})

func TryRunUpAndVerifyResult(db *sql.DB, migrator migration.Migrator, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()

	err := migrator.Up()
	Expect(err).NotTo(HaveOccurred())

	ExpectSchemaMigrationsTableToHaveVersion(db, initialSchemaVersion)

	ExpectMigrationVersionTableNotToExist(db)

	ExpectToBeAbleToInsertData(db)
}

func TryRunMigrateAndVerifyResult(db *sql.DB, migrator migration.Migrator, version int, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()

	err := migrator.Migrate(version)
	Expect(err).NotTo(HaveOccurred())

	ExpectSchemaMigrationsTableToHaveVersion(db, version)

	ExpectMigrationVersionTableNotToExist(db)

	ExpectToBeAbleToInsertData(db)
}

func SetupMigrationVersionTableToExistAtVersion(db *sql.DB, version int) {
	_, err := db.Exec(`CREATE TABLE migration_version(version int)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO migration_version(version) VALUES($1)`, version)
	Expect(err).NotTo(HaveOccurred())
}

func SetupSchemaMigrationsTableToExistAtVersion(db *sql.DB, version int) {
	SetupSchemaMigrationsTableToExistAtVersionWithDirtyState(db, version, false)
}

func SetupSchemaMigrationsTableToExistAtVersionWithDirtyState(db *sql.DB, version int, dirty bool) {
	_, err := db.Exec(`CREATE TABLE schema_migrations(version bigint, dirty boolean)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO schema_migrations(version, dirty) VALUES($1, $2)`, version, dirty)
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
