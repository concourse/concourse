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
	"github.com/concourse/atc/db/migration/migrationfakes"
	"github.com/lib/pq"

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
		bindata     *migrationfakes.FakeBindata
	)

	BeforeEach(func() {
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockFactory = lock.NewLockFactory(lockDB)

		strategy = encryption.NewNoEncryption()
		bindata = new(migrationfakes.FakeBindata)
		bindata.AssetStub = asset
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
			bindata.AssetNamesReturns([]string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"2000000000_latest_migration_does_not_matter.up.sql",
			})
			bindata.AssetStub = func(name string) ([]byte, error) {
				if name == "1000_some_migration.up.sql" {
					return []byte{}, nil
				} else if name == "2000000000_latest_migration_does_not_matter.up.sql" {
					return []byte{}, nil
				}
				return asset(name)
			}

			myDatabaseVersion := 1234567890

			SetupMigrationsHistoryTableToExistAtVersion(db, myDatabaseVersion)

			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

			version, err := migrator.CurrentVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(myDatabaseVersion))
		})

		It("SupportedVersion reports the highest supported migration version", func() {

			SetupMigrationsHistoryTableToExistAtVersion(db, initialSchemaVersion)

			bindata.AssetNamesReturns([]string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"300000_this_is_to_prove_we_dont_use_string_sort.up.sql",
				"2000000000_latest_migration.up.sql",
			})
			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

			version, err := migrator.SupportedVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(2000000000))
		})

		It("Ignores files it can't parse", func() {

			SetupMigrationsHistoryTableToExistAtVersion(db, initialSchemaVersion)

			bindata.AssetNamesReturns([]string{
				"1000_some_migration.up.sql",
				"1510262030_initial_schema.up.sql",
				"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				"300000_this_is_to_prove_we_dont_use_string_sort.up.sql",
				"2000000000_latest_migration.up.sql",
				"migrations.go",
			})
			migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

			version, err := migrator.SupportedVersion()
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(2000000000))
		})
	})

	Context("Upgrade", func() {
		Context("old schema_migrations table exist", func() {
			var dirty bool

			JustBeforeEach(func() {
				SetupSchemaMigrationsTable(db, 8878, dirty)
			})

			Context("dirty state is true", func() {
				BeforeEach(func() {
					dirty = true
				})
				It("errors", func() {

					Expect(err).NotTo(HaveOccurred())

					migrator := migration.NewMigrator(db, lockFactory, strategy)

					err = migrator.Up()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Database is in a dirty state"))

					var newTableCreated bool
					err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name='migrations_history')").Scan(&newTableCreated)
					Expect(newTableCreated).To(BeFalse())
				})
			})

			Context("dirty state is false", func() {
				BeforeEach(func() {
					dirty = false
				})

				It("populate migrations_history table with starting version from schema_migrations table", func() {
					startTime := time.Now()
					migrator := migration.NewMigrator(db, lockFactory, strategy)

					err = migrator.Up()
					Expect(err).NotTo(HaveOccurred())

					var (
						version   int
						isDirty   bool
						timeStamp pq.NullTime
						status    string
						direction string
					)
					err = db.QueryRow("SELECT * from migrations_history ORDER BY tstamp ASC LIMIT 1").Scan(&version, &timeStamp, &direction, &status, &isDirty)
					Expect(version).To(Equal(8878))
					Expect(isDirty).To(BeFalse())
					Expect(timeStamp.Time.After(startTime)).To(Equal(true))
					Expect(direction).To(Equal("up"))
					Expect(status).To(Equal("passed"))
				})

				Context("when the migrations_history table already exists", func() {
					It("does not repopulate the migrations_history table", func() {
						SetupMigrationsHistoryTableToExistAtVersion(db, 8878)
						startTime := time.Now()
						migrator := migration.NewMigrator(db, lockFactory, strategy)

						err = migrator.Up()
						Expect(err).NotTo(HaveOccurred())

						var timeStamp pq.NullTime
						rows, err := db.Query("SELECT tstamp FROM migrations_history WHERE version=8878")
						Expect(err).NotTo(HaveOccurred())
						var numRows = 0
						for rows.Next() {
							err = rows.Scan(&timeStamp)
							numRows++
						}
						Expect(numRows).To(Equal(1))
						Expect(timeStamp.Time.Before(startTime)).To(Equal(true))
					})
				})
			})
		})

		Context("sql migrations", func() {
			It("runs a migration", func() {
				simpleMigrationFilename := "1000_test_table_created.up.sql"
				bindata.AssetReturns([]byte(`
						BEGIN;
						CREATE TABLE some_table (id integer);
						COMMIT;
						`), nil)

				bindata.AssetNamesReturns([]string{
					simpleMigrationFilename,
				})

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				migrations, err := migrator.Migrations()
				Expect(err).NotTo(HaveOccurred())
				Expect(len(migrations)).To(Equal(1))

				err = migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				By("Creating the table in the database")
				var exists string
				err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables where table_name = 'some_table')").Scan(&exists)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(Equal("true"))

				By("Updating the migrations_history table")
				ExpectDatabaseMigrationVersionToEqual(migrator, 1000)
			})

			It("ignores migrations before the current version", func() {
				SetupMigrationsHistoryTableToExistAtVersion(db, 1000)

				simpleMigrationFilename := "1000_test_table_created.up.sql"
				bindata.AssetStub = func(name string) ([]byte, error) {
					if name == simpleMigrationFilename {
						return []byte(`
						BEGIN;
						CREATE TABLE some_table (id integer);
						COMMIT;
						`), nil
					}
					return asset(name)
				}
				bindata.AssetNamesReturns([]string{
					simpleMigrationFilename,
				})

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				By("Not creating the database referenced in the migration")
				var exists string
				err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables where table_name = 'some_table')").Scan(&exists)
				Expect(err).NotTo(HaveOccurred())
				Expect(exists).To(Equal("false"))
			})

			It("runs the up migrations in ascending order", func() {
				addTableMigrationFilename := "1000_test_table_created.up.sql"
				removeTableMigrationFilename := "1001_test_table_created.up.sql"

				bindata.AssetStub = func(name string) ([]byte, error) {
					if name == addTableMigrationFilename {
						return []byte(`
						BEGIN;
						CREATE TABLE some_table (id integer);
						COMMIT;
						`), nil
					} else if name == removeTableMigrationFilename {
						return []byte(`
						BEGIN;
						DROP TABLE some_table;
						COMMIT;
						`), nil
					}
					return asset(name)
				}

				bindata.AssetNamesReturns([]string{
					removeTableMigrationFilename,
					addTableMigrationFilename,
				})

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

			})

			Context("With a transactional migration", func() {
				It("leaves the database clean after a failure", func() {
					bindata.AssetNamesReturns([]string{
						"1510262030_initial_schema.up.sql",
						"1525724789_drop_reaper_addr_from_workers.up.sql",
					})
					migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

					err := migrator.Up()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("rolled back the migration"))
					ExpectDatabaseMigrationVersionToEqual(migrator, initialSchemaVersion)
					ExpectMigrationToHaveFailed(db, 1525724789, false)
				})
			})

			Context("With a non-transactional migration", func() {
				It("fails if the migration version is in a dirty state", func() {
					dirtyMigrationFilename := "1510262031_dirty_migration.up.sql"
					bindata.AssetStub = func(name string) ([]byte, error) {
						if name == dirtyMigrationFilename {
							return []byte(`
							-- NO_TRANSACTION
							DROP TABLE nonexistent;
						`), nil
						}
						return asset(name)
					}

					bindata.AssetNamesReturns([]string{
						dirtyMigrationFilename,
					})

					migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

					err := migrator.Up()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(MatchRegexp("Migration.*failed"))

					ExpectMigrationToHaveFailed(db, 1510262031, true)
				})
			})

			It("Doesn't fail if there are no migrations to run", func() {
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
				})

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				err = migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				ExpectDatabaseMigrationVersionToEqual(migrator, initialSchemaVersion)

				ExpectMigrationVersionTableNotToExist(db)

				ExpectToBeAbleToInsertData(db)
			})

			It("Locks the database so multiple ATCs don't all run migrations at the same time", func() {
				SetupMigrationsHistoryTableToExistAtVersion(db, 1510262030)

				SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
				})
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				var wg sync.WaitGroup
				wg.Add(3)

				go TryRunUpAndVerifyResult(db, migrator, &wg)
				go TryRunUpAndVerifyResult(db, migrator, &wg)
				go TryRunUpAndVerifyResult(db, migrator, &wg)

				wg.Wait()
			})
		})

		Context("golang migrations", func() {
			It("runs a migration with Migrate", func() {

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1516643303_update_auth_providers.up.go",
				})

				By("applying the initial migration")
				err := migrator.Migrate(1510262030)
				var columnExists string
				err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.columns where table_name = 'teams' AND column_name='basic_auth')").Scan(&columnExists)
				Expect(err).NotTo(HaveOccurred())
				Expect(columnExists).To(Equal("true"))

				err = migrator.Migrate(1516643303)
				Expect(err).NotTo(HaveOccurred())

				By("applying the go migration")
				err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.columns where table_name = 'teams' AND column_name='basic_auth')").Scan(&columnExists)
				Expect(err).NotTo(HaveOccurred())
				Expect(columnExists).To(Equal("false"))

				By("updating the schema migrations table")
				ExpectDatabaseMigrationVersionToEqual(migrator, 1516643303)
			})

			It("runs a migration with Up", func() {

				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1516643303_update_auth_providers.up.go",
				})

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				By("applying the migration")
				var columnExists string
				err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.columns where table_name = 'teams' AND column_name='basic_auth')").Scan(&columnExists)
				Expect(err).NotTo(HaveOccurred())
				Expect(columnExists).To(Equal("false"))

				By("updating the schema migrations table")
				ExpectDatabaseMigrationVersionToEqual(migrator, 1516643303)
			})
		})
	})

	Context("Downgrade", func() {
		Context("Downgrades to a version that uses the old mattes/migrate schema_migrations table", func() {
			It("Downgrades to a given version and write it to a new created schema_migrations table", func() {
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.down.sql",
				})
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err := migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(upgradedSchemaVersion))

				err = migrator.Migrate(initialSchemaVersion)
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err = migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(initialSchemaVersion))

				ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "schema_migrations")

				ExpectToBeAbleToInsertData(db)
			})

			It("Downgrades to a given version and write it to the existing schema_migrations table with dirty true", func() {

				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.down.sql",
				})
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err := migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(upgradedSchemaVersion))

				SetupSchemaMigrationsTable(db, 8878, true)

				err = migrator.Migrate(initialSchemaVersion)
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err = migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(initialSchemaVersion))

				ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "schema_migrations")

				ExpectToBeAbleToInsertData(db)
			})
		})

		Context("Downgrades to a version with new migrations_history table", func() {
			It("Downgrades to a given version", func() {
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.down.sql",
				})
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				err := migrator.Up()
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err := migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(upgradedSchemaVersion))

				err = migrator.Migrate(initialSchemaVersion)
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err = migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(initialSchemaVersion))

				ExpectToBeAbleToInsertData(db)
			})

			It("Doesn't fail if already at the requested version", func() {
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.up.sql",
				})
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)

				err := migrator.Migrate(upgradedSchemaVersion)
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err := migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(upgradedSchemaVersion))

				err = migrator.Migrate(upgradedSchemaVersion)
				Expect(err).NotTo(HaveOccurred())

				currentVersion, err = migrator.CurrentVersion()
				Expect(err).NotTo(HaveOccurred())
				Expect(currentVersion).To(Equal(upgradedSchemaVersion))

				ExpectToBeAbleToInsertData(db)
			})

			It("Locks the database so multiple consumers don't run downgrade at the same time", func() {
				migrator := migration.NewMigratorForMigrations(db, lockFactory, strategy, bindata)
				bindata.AssetNamesReturns([]string{
					"1510262030_initial_schema.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.up.sql",
					"1510670987_update_unique_constraint_for_resource_caches.down.sql",
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

})

func TryRunUpAndVerifyResult(db *sql.DB, migrator migration.Migrator, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()

	err := migrator.Up()
	Expect(err).NotTo(HaveOccurred())

	ExpectDatabaseMigrationVersionToEqual(migrator, initialSchemaVersion)

	ExpectToBeAbleToInsertData(db)
}

func TryRunMigrateAndVerifyResult(db *sql.DB, migrator migration.Migrator, version int, wg *sync.WaitGroup) {
	defer GinkgoRecover()
	defer wg.Done()

	err := migrator.Migrate(version)
	Expect(err).NotTo(HaveOccurred())

	ExpectDatabaseMigrationVersionToEqual(migrator, version)

	ExpectToBeAbleToInsertData(db)
}

func SetupMigrationsHistoryTableToExistAtVersion(db *sql.DB, version int) {
	_, err := db.Exec(`CREATE TABLE migrations_history(version bigint, tstamp timestamp with time zone, direction varchar, status varchar, dirty boolean)`)
	Expect(err).NotTo(HaveOccurred())

	_, err = db.Exec(`INSERT INTO migrations_history(version, tstamp, direction, status, dirty) VALUES($1, current_timestamp, 'up', 'passed', false)`, version)
	Expect(err).NotTo(HaveOccurred())
}

func SetupSchemaMigrationsTable(db *sql.DB, version int, dirty bool) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version bigint, dirty boolean)")
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2)", version, dirty)
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

func ExpectDatabaseMigrationVersionToEqual(migrator migration.Migrator, expectedVersion int) {
	var dbVersion int
	dbVersion, err := migrator.CurrentVersion()
	Expect(err).NotTo(HaveOccurred())
	Expect(dbVersion).To(Equal(expectedVersion))
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

func ExpectMigrationToHaveFailed(dbConn *sql.DB, failedVersion int, expectDirty bool) {
	var status string
	var dirty bool
	err := dbConn.QueryRow("SELECT status, dirty FROM migrations_history WHERE version=$1 ORDER BY tstamp desc LIMIT 1", failedVersion).Scan(&status, &dirty)
	Expect(err).NotTo(HaveOccurred())
	Expect(status).To(Equal("failed"))
	Expect(dirty).To(Equal(expectDirty))
}
