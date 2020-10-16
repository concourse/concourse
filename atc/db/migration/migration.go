package migration

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration/migrations"
	"github.com/gobuffalo/packr"
	multierror "github.com/hashicorp/go-multierror"
	_ "github.com/lib/pq"
)

func NewOpenHelper(driver, name string, lockFactory lock.LockFactory, newKey *encryption.Key, oldKey *encryption.Key) *OpenHelper {
	return &OpenHelper{
		driver,
		name,
		lockFactory,
		newKey,
		oldKey,
	}
}

type OpenHelper struct {
	driver         string
	dataSourceName string
	lockFactory    lock.LockFactory
	newKey         *encryption.Key
	oldKey         *encryption.Key
}

func (self *OpenHelper) CurrentVersion() (int, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, self.lockFactory).CurrentVersion()
}

func (self *OpenHelper) SupportedVersion() (int, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, self.lockFactory).SupportedVersion()
}

func (self *OpenHelper) Open() (*sql.DB, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := NewMigrator(db, self.lockFactory).Up(self.newKey, self.oldKey); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (self *OpenHelper) OpenAtVersion(version int) (*sql.DB, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := NewMigrator(db, self.lockFactory).Migrate(self.newKey, self.oldKey, version); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (self *OpenHelper) MigrateToVersion(version int) error {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return err
	}

	defer db.Close()
	m := NewMigrator(db, self.lockFactory)

	err = self.migrateFromMigrationVersion(db)
	if err != nil {
		return err
	}

	return m.Migrate(self.newKey, self.oldKey, version)
}

func (self *OpenHelper) migrateFromMigrationVersion(db *sql.DB) error {

	legacySchemaExists, err := checkTableExist(db, "migration_version")
	if err != nil {
		return err
	}

	if !legacySchemaExists {
		return nil
	}

	oldMigrationLastVersion := 189
	newMigrationStartVersion := 1510262030

	var dbVersion int

	if err = db.QueryRow("SELECT version FROM migration_version").Scan(&dbVersion); err != nil {
		return err
	}

	if dbVersion != oldMigrationLastVersion {
		return fmt.Errorf("Must upgrade from db version %d (concourse 3.6.0), current db version: %d", oldMigrationLastVersion, dbVersion)
	}

	if _, err = db.Exec("DROP TABLE IF EXISTS migration_version"); err != nil {
		return err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version bigint, dirty boolean)")
	if err != nil {
		return err
	}

	_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", newMigrationStartVersion)
	if err != nil {
		return err
	}

	return nil
}

type Migrator interface {
	CurrentVersion() (int, error)
	SupportedVersion() (int, error)
	Migrate(newKey, oldKey *encryption.Key, version int) error
	Up(newKey, oldKey *encryption.Key) error
	Migrations() ([]migration, error)
}

func NewMigrator(db *sql.DB, lockFactory lock.LockFactory) Migrator {
	return NewMigratorForMigrations(db, lockFactory, &packrSource{packr.NewBox("./migrations")})
}

func NewMigratorForMigrations(db *sql.DB, lockFactory lock.LockFactory, bindata Bindata) Migrator {
	return &migrator{
		db,
		lockFactory,
		lager.NewLogger("migrations"),
		bindata,
	}
}

type migrator struct {
	db          *sql.DB
	lockFactory lock.LockFactory
	logger      lager.Logger
	bindata     Bindata
}

func (m *migrator) SupportedVersion() (int, error) {
	matches := []migration{}

	assets := m.bindata.AssetNames()

	var parser = NewParser(m.bindata)
	for _, match := range assets {
		if migration, err := parser.ParseMigrationFilename(match); err == nil {
			matches = append(matches, migration)
		}
	}
	sortMigrations(matches)
	return matches[len(matches)-1].Version, nil
}

func (self *migrator) CurrentVersion() (int, error) {
	var currentVersion int
	var direction string
	err := self.db.QueryRow("SELECT version, direction FROM migrations_history WHERE status!='failed' ORDER BY tstamp DESC LIMIT 1").Scan(&currentVersion, &direction)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return -1, err
	}
	migrations, err := self.Migrations()
	if err != nil {
		return -1, err
	}
	versions := []int{migrations[0].Version}
	for _, m := range migrations {
		if m.Version > versions[len(versions)-1] {
			versions = append(versions, m.Version)
		}
	}
	for i, version := range versions {
		if currentVersion == version && direction == "down" {
			currentVersion = versions[i-1]
			break
		}
	}
	return currentVersion, nil
}

func (self *migrator) Migrate(newKey, oldKey *encryption.Key, toVersion int) error {
	var strategy encryption.Strategy
	if oldKey != nil {
		strategy = oldKey
	} else if newKey != nil {
		// special case - if the old key is not provided but the new key is,
		// this might mean the data was not encrypted, or that it was encrypted with newKey
		strategy = encryption.NewFallbackStrategy(newKey, encryption.NewNoEncryption())
	} else if newKey == nil {
		strategy = encryption.NewNoEncryption()
	}

	lock, err := self.acquireLock()
	if err != nil {
		return err
	}

	if lock != nil {
		defer lock.Release()
	}

	existingDBVersion, err := self.migrateFromSchemaMigrations()
	if err != nil {
		return err
	}

	_, err = self.db.Exec("CREATE TABLE IF NOT EXISTS migrations_history (version bigint, tstamp timestamp with time zone, direction varchar, status varchar, dirty boolean)")
	if err != nil {
		return err
	}

	if existingDBVersion > 0 {
		var containsOldMigrationInfo bool
		err = self.db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations_history where version=$1)", existingDBVersion).Scan(&containsOldMigrationInfo)
		if err != nil {
			return err
		}

		if !containsOldMigrationInfo {
			_, err = self.db.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, 'up', 'passed', false)", existingDBVersion)
			if err != nil {
				return err
			}
		}
	}

	currentVersion, err := self.CurrentVersion()
	if err != nil {
		return err
	}

	migrations, err := self.Migrations()
	if err != nil {
		return err
	}

	if currentVersion <= toVersion {
		for _, m := range migrations {
			if currentVersion < m.Version && m.Version <= toVersion && m.Direction == "up" {
				err = self.runMigration(m, strategy)
				if err != nil {
					return err
				}
			}
		}
	} else {
		for i := len(migrations) - 1; i >= 0; i-- {
			if currentVersion >= migrations[i].Version && migrations[i].Version > toVersion && migrations[i].Direction == "down" {
				err = self.runMigration(migrations[i], strategy)
				if err != nil {
					return err
				}

			}
		}

		err = self.migrateToSchemaMigrations(toVersion)
		if err != nil {
			return err
		}
	}

	switch {
	case oldKey != nil && newKey == nil:
		err = self.decryptToPlaintext(oldKey)
	case oldKey != nil && newKey != nil:
		err = self.encryptWithNewKey(newKey, oldKey)
	}
	if err != nil {
		return err
	}

	if newKey != nil {
		err = self.encryptPlaintext(newKey)
		if err != nil {
			return err
		}
	}

	return nil
}

type Strategy int

const (
	GoMigration Strategy = iota
	SQLTransaction
	SQLNoTransaction
)

type migration struct {
	Name       string
	Version    int
	Direction  string
	Statements []string
	Strategy   Strategy
}

func (m *migrator) recordMigrationFailure(migration migration, err error, dirty bool) error {
	_, dbErr := m.db.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'failed', $3)", migration.Version, migration.Direction, dirty)
	return multierror.Append(fmt.Errorf("Migration '%s' failed: %v", migration.Name, err), dbErr)
}

func (m *migrator) runMigration(migration migration, strategy encryption.Strategy) error {
	var err error

	switch migration.Strategy {
	case GoMigration:
		err = migrations.NewMigrations(m.db, strategy).Run(migration.Name)
		if err != nil {
			return m.recordMigrationFailure(migration, err, false)
		}
	case SQLTransaction:
		tx, err := m.db.Begin()
		for _, statement := range migration.Statements {
			_, err = tx.Exec(statement)
			if err != nil {
				err = multierror.Append(fmt.Errorf("Transaction %v failed, rolled back the migration", statement), err)
				txErr := tx.Rollback()
				if txErr != nil {
					err = multierror.Append(fmt.Errorf("Rolling back transaction %v failed", statement), txErr)
				}
				return m.recordMigrationFailure(migration, err, false)
			}
		}
		err = tx.Commit()
	case SQLNoTransaction:
		for _, statement := range migration.Statements {
			_, err = m.db.Exec(statement)
			if err != nil {
				return m.recordMigrationFailure(migration, err, true)
			}
		}
	}

	_, err = m.db.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'passed', false)", migration.Version, migration.Direction)
	return err
}

func (self *migrator) Migrations() ([]migration, error) {
	migrationList := []migration{}
	assets := self.bindata.AssetNames()
	var parser = NewParser(self.bindata)
	for _, assetName := range assets {
		parsedMigration, err := parser.ParseFileToMigration(assetName)
		if err != nil {
			return nil, err
		}
		migrationList = append(migrationList, parsedMigration)
	}

	sortMigrations(migrationList)

	return migrationList, nil
}

func (self *migrator) Up(newKey, oldKey *encryption.Key) error {
	migrations, err := self.Migrations()
	if err != nil {
		return err
	}
	return self.Migrate(newKey, oldKey, migrations[len(migrations)-1].Version)
}

func (self *migrator) acquireLock() (lock.Lock, error) {

	var err error
	var acquired bool
	var newLock lock.Lock

	if self.lockFactory != nil {
		for {
			newLock, acquired, err = self.lockFactory.Acquire(self.logger, lock.NewDatabaseMigrationLockID())

			if err != nil {
				return nil, err
			}

			if acquired {
				break
			}

			time.Sleep(1 * time.Second)
		}
	}

	return newLock, err
}

func checkTableExist(db *sql.DB, tableName string) (bool, error) {
	var existingTable sql.NullString
	err := db.QueryRow("SELECT to_regclass($1)", tableName).Scan(&existingTable)
	if err != nil {
		return false, err
	}

	return existingTable.Valid, nil
}

func (self *migrator) migrateFromSchemaMigrations() (int, error) {
	oldSchemaExists, err := checkTableExist(self.db, "schema_migrations")
	if err != nil {
		return 0, err
	}

	newSchemaExists, err := checkTableExist(self.db, "migrations_history")
	if err != nil {
		return 0, err
	}

	if !oldSchemaExists || newSchemaExists {
		return 0, nil
	}

	var isDirty = false
	var existingVersion int
	err = self.db.QueryRow("SELECT dirty, version FROM schema_migrations LIMIT 1").Scan(&isDirty, &existingVersion)
	if err != nil {
		return 0, err
	}

	if isDirty {
		return 0, errors.New("cannot begin migration. Database is in a dirty state")
	}

	return existingVersion, nil
}

func sortMigrations(migrationList []migration) {
	sort.Slice(migrationList, func(i, j int) bool {
		return migrationList[i].Version < migrationList[j].Version
	})
}

func (self *migrator) migrateToSchemaMigrations(toVersion int) error {
	newMigrationsHistoryFirstVersion := 1532706545

	if toVersion >= newMigrationsHistoryFirstVersion {
		return nil
	}

	oldSchemaExists, err := checkTableExist(self.db, "schema_migrations")
	if err != nil {
		return err
	}

	if !oldSchemaExists {
		_, err := self.db.Exec("CREATE TABLE schema_migrations (version bigint, dirty boolean)")
		if err != nil {
			return err
		}

		_, err = self.db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", toVersion)
		if err != nil {
			return err
		}
	} else {
		_, err := self.db.Exec("UPDATE schema_migrations SET version=$1, dirty=false", toVersion)
		if err != nil {
			return err
		}
	}

	return nil
}
