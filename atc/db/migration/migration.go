package migration

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration/migrations"
	multierror "github.com/hashicorp/go-multierror"
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

func (helper *OpenHelper) CurrentVersion() (int, error) {
	db, err := sql.Open(helper.driver, helper.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, helper.lockFactory).CurrentVersion()
}

func (helper *OpenHelper) SupportedVersion() (int, error) {
	db, err := sql.Open(helper.driver, helper.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, helper.lockFactory).SupportedVersion()
}

func (helper *OpenHelper) Open() (*sql.DB, error) {
	db, err := sql.Open(helper.driver, helper.dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := NewMigrator(db, helper.lockFactory).Up(helper.newKey, helper.oldKey); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (helper *OpenHelper) OpenAtVersion(version int) (*sql.DB, error) {
	db, err := sql.Open(helper.driver, helper.dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := NewMigrator(db, helper.lockFactory).Migrate(helper.newKey, helper.oldKey, version); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func (helper *OpenHelper) MigrateToVersion(version int) error {
	db, err := sql.Open(helper.driver, helper.dataSourceName)
	if err != nil {
		return err
	}

	defer db.Close()
	m := NewMigrator(db, helper.lockFactory)

	err = helper.migrateFromMigrationVersion(db)
	if err != nil {
		return err
	}

	return m.Migrate(helper.newKey, helper.oldKey, version)
}

func (helper *OpenHelper) migrateFromMigrationVersion(db *sql.DB) error {

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
		return fmt.Errorf("must upgrade from db version %d (concourse 3.6.0), current db version: %d", oldMigrationLastVersion, dbVersion)
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

//go:embed migrations
var migrationsEmbed embed.FS

func NewMigrator(db *sql.DB, lockFactory lock.LockFactory) Migrator {
	migrationsFS, err := fs.Sub(migrationsEmbed, "migrations")
	if err != nil {
		// impossible due to const value arg
		panic(err)
	}

	return NewMigratorForMigrations(db, lockFactory, migrationsFS)
}

func NewMigratorForMigrations(db *sql.DB, lockFactory lock.LockFactory, migrationsFS fs.FS) Migrator {
	return &migrator{
		db,
		lockFactory,
		lager.NewLogger("migrations"),
		migrationsFS,
	}
}

type migrator struct {
	db           *sql.DB
	lockFactory  lock.LockFactory
	logger       lager.Logger
	migrationsFS fs.FS
}

func (helper *migrator) Migrations() ([]migration, error) {
	migrationList := []migration{}

	assets, err := fs.ReadDir(helper.migrationsFS, ".")
	if err != nil {
		return nil, err
	}

	var parser = NewParser(helper.migrationsFS)
	for _, asset := range assets {
		if asset.Name() == "migrations.go" {
			// special file declaring type for Go migrations
			continue
		}

		parsedMigration, err := parser.ParseFileToMigration(asset.Name())
		if err != nil {
			return nil, fmt.Errorf("parse migration filename %s: %w", asset.Name(), err)
		}

		migrationList = append(migrationList, parsedMigration)
	}

	sortMigrations(migrationList)

	return migrationList, nil
}

func (m *migrator) SupportedVersion() (int, error) {
	migrations, err := m.Migrations()
	if err != nil {
		return 0, fmt.Errorf("list migrations: %w", err)
	}

	if len(migrations) == 0 {
		return 0, fmt.Errorf("no migrations")
	}

	return migrations[len(migrations)-1].Version, nil
}

func (helper *migrator) CurrentVersion() (int, error) {
	var currentVersion int
	var direction string
	err := helper.db.QueryRow("SELECT version, direction FROM migrations_history WHERE status!='failed' ORDER BY tstamp DESC LIMIT 1").Scan(&currentVersion, &direction)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return -1, err
	}
	migrations, err := helper.Migrations()
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

func (helper *migrator) Migrate(newKey, oldKey *encryption.Key, toVersion int) error {
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

	lock, err := helper.acquireLock()
	if err != nil {
		return err
	}

	if lock != nil {
		defer lock.Release()
	}

	existingDBVersion, err := helper.migrateFromSchemaMigrations()
	if err != nil {
		return err
	}

	_, err = helper.db.Exec("CREATE TABLE IF NOT EXISTS migrations_history (version bigint, tstamp timestamp with time zone, direction varchar, status varchar, dirty boolean)")
	if err != nil {
		return err
	}

	if existingDBVersion > 0 {
		var containsOldMigrationInfo bool
		err = helper.db.QueryRow("SELECT EXISTS (SELECT 1 FROM migrations_history where version=$1)", existingDBVersion).Scan(&containsOldMigrationInfo)
		if err != nil {
			return err
		}

		if !containsOldMigrationInfo {
			_, err = helper.db.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, 'up', 'passed', false)", existingDBVersion)
			if err != nil {
				return err
			}
		}
	}

	currentVersion, err := helper.CurrentVersion()
	if err != nil {
		return err
	}

	migrations, err := helper.Migrations()
	if err != nil {
		return err
	}

	if currentVersion <= toVersion {
		for _, m := range migrations {
			if currentVersion < m.Version && m.Version <= toVersion && m.Direction == "up" {
				err = helper.runMigration(m, strategy)
				if err != nil {
					return err
				}
			}
		}
	} else {
		for i := len(migrations) - 1; i >= 0; i-- {
			if currentVersion >= migrations[i].Version && migrations[i].Version > toVersion && migrations[i].Direction == "down" {
				err = helper.runMigration(migrations[i], strategy)
				if err != nil {
					return err
				}

			}
		}

		err = helper.migrateToSchemaMigrations(toVersion)
		if err != nil {
			return err
		}
	}

	switch {
	case oldKey != nil && newKey == nil:
		err = helper.decryptToPlaintext(oldKey)
	case oldKey != nil && newKey != nil:
		err = helper.encryptWithNewKey(newKey, oldKey)
	}
	if err != nil {
		return err
	}

	if newKey != nil {
		err = helper.encryptPlaintext(newKey)
		if err != nil {
			return err
		}
	}

	return nil
}

type Strategy int

const (
	GoMigration Strategy = iota
	SQLMigration
)

type migration struct {
	Name       string
	Version    int
	Direction  string
	Statements string
	Strategy   Strategy
}

func (m *migrator) recordMigrationFailure(migration migration, migrationErr error, dirty bool) error {
	_, recordErr := m.db.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'failed', $3)", migration.Version, migration.Direction, dirty)
	if recordErr != nil {
		return multierror.Append(
			migrationErr,
			fmt.Errorf("record failure to migration history: %w", recordErr),
		)
	}

	return migrationErr
}

func (m *migrator) runMigration(migration migration, strategy encryption.Strategy) (err error) {
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = m.recordMigrationFailure(
				migration,
				fmt.Errorf("migration '%s' failed and was rolled back: %w", migration.Name, err),
				false,
			)

			rbErr := tx.Rollback()
			if rbErr != nil {
				err = multierror.Append(err, fmt.Errorf("rollback failed: %w", rbErr))
			}
		}
	}()

	switch migration.Strategy {
	case GoMigration:
		err = migrations.NewMigrations(tx, strategy).Run(migration.Name)
		if err != nil {
			return err
		}
	case SQLMigration:
		_, err = tx.Exec(migration.Statements)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec("INSERT INTO migrations_history (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'passed', false)", migration.Version, migration.Direction)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (helper *migrator) Up(newKey, oldKey *encryption.Key) error {
	migrations, err := helper.Migrations()
	if err != nil {
		return err
	}
	return helper.Migrate(newKey, oldKey, migrations[len(migrations)-1].Version)
}

func (helper *migrator) acquireLock() (lock.Lock, error) {

	var err error
	var acquired bool
	var newLock lock.Lock

	if helper.lockFactory != nil {
		for {
			newLock, acquired, err = helper.lockFactory.Acquire(helper.logger, lock.NewDatabaseMigrationLockID())

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

func (helper *migrator) migrateFromSchemaMigrations() (int, error) {
	oldSchemaExists, err := checkTableExist(helper.db, "schema_migrations")
	if err != nil {
		return 0, err
	}

	newSchemaExists, err := checkTableExist(helper.db, "migrations_history")
	if err != nil {
		return 0, err
	}

	if !oldSchemaExists || newSchemaExists {
		return 0, nil
	}

	var isDirty = false
	var existingVersion int
	err = helper.db.QueryRow("SELECT dirty, version FROM schema_migrations LIMIT 1").Scan(&isDirty, &existingVersion)
	if err != nil {
		return 0, err
	}

	if isDirty {
		return 0, errors.New("cannot begin migration: database is in a dirty state")
	}

	return existingVersion, nil
}

func sortMigrations(migrationList []migration) {
	sort.Slice(migrationList, func(i, j int) bool {
		return migrationList[i].Version < migrationList[j].Version
	})
}

func (helper *migrator) migrateToSchemaMigrations(toVersion int) error {
	newMigrationsHistoryFirstVersion := 1532706545

	if toVersion >= newMigrationsHistoryFirstVersion {
		return nil
	}

	oldSchemaExists, err := checkTableExist(helper.db, "schema_migrations")
	if err != nil {
		return err
	}

	if !oldSchemaExists {
		_, err := helper.db.Exec("CREATE TABLE schema_migrations (version bigint, dirty boolean)")
		if err != nil {
			return err
		}

		_, err = helper.db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)", toVersion)
		if err != nil {
			return err
		}
	} else {
		_, err := helper.db.Exec("UPDATE schema_migrations SET version=$1, dirty=false", toVersion)
		if err != nil {
			return err
		}
	}

	return nil
}
