package migration

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migration/migrations"
	multierror "github.com/hashicorp/go-multierror"
	_ "github.com/lib/pq"
	"github.com/mattes/migrate/source"
	_ "github.com/mattes/migrate/source/file"
)

func NewOpenHelper(driver, name string, lockFactory lock.LockFactory, strategy encryption.Strategy) *OpenHelper {
	return &OpenHelper{
		driver,
		name,
		lockFactory,
		strategy,
	}
}

type OpenHelper struct {
	driver         string
	dataSourceName string
	lockFactory    lock.LockFactory
	strategy       encryption.Strategy
}

func (self *OpenHelper) CurrentVersion() (int, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, self.lockFactory, self.strategy).CurrentVersion()
}

func (self *OpenHelper) SupportedVersion() (int, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return -1, err
	}

	defer db.Close()

	return NewMigrator(db, self.lockFactory, self.strategy).SupportedVersion()
}

func (self *OpenHelper) Open() (*sql.DB, error) {
	db, err := sql.Open(self.driver, self.dataSourceName)
	if err != nil {
		return nil, err
	}

	if err := NewMigrator(db, self.lockFactory, self.strategy).Up(); err != nil {
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

	if err := NewMigrator(db, self.lockFactory, self.strategy).Migrate(version); err != nil {
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

	if err := NewMigrator(db, self.lockFactory, self.strategy).Migrate(version); err != nil {
		return err
	}

	return nil
}

type Migrator interface {
	CurrentVersion() (int, error)
	SupportedVersion() (int, error)
	Migrate(version int) error
	Up() error
	Migrations() ([]migration, error)
}

func NewMigrator(db *sql.DB, lockFactory lock.LockFactory, strategy encryption.Strategy) Migrator {
	return NewMigratorForMigrations(db, lockFactory, strategy, &bindataSource{})
}

func NewMigratorForMigrations(db *sql.DB, lockFactory lock.LockFactory, strategy encryption.Strategy, bindata Bindata) Migrator {
	return &migrator{
		db,
		lockFactory,
		strategy,
		lager.NewLogger("migrations"),
		bindata,
	}
}

type migrator struct {
	db          *sql.DB
	lockFactory lock.LockFactory
	strategy    encryption.Strategy
	logger      lager.Logger
	bindata     Bindata
}

func (self *migrator) SupportedVersion() (int, error) {

	latest := filenames(self.bindata.AssetNames()).Latest()

	m, err := source.Parse(latest)
	if err != nil {
		return -1, err
	}

	return int(m.Version), nil
}

func (self *migrator) CurrentVersion() (int, error) {
	var currentVersion int
	var direction string
	err := self.db.QueryRow("SELECT version, direction FROM schema_migrations WHERE status!='failed' ORDER BY tstamp DESC LIMIT 1").Scan(&currentVersion, &direction)
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

func (self *migrator) Migrate(toVersion int) error {

	lock, err := self.acquireLock()
	if err != nil {
		return err
	}

	if lock != nil {
		defer lock.Release()
	}

	_, err = self.db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version bigint, tstamp timestamp with time zone, direction varchar, status varchar, dirty boolean)")
	if err != nil {
		return err
	}
	err = self.convertLegacySchemaTableToCurrent()
	if err != nil {
		return err
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
				err = self.runMigration(m)
				if err != nil {
					return err
				}
			}
		}
	} else {
		for i := len(migrations) - 1; i >= 0; i-- {
			if currentVersion >= migrations[i].Version && migrations[i].Version > toVersion && migrations[i].Direction == "down" {
				err = self.runMigration(migrations[i])
				if err != nil {
					return err
				}
			}
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
	_, dbErr := m.db.Exec("INSERT INTO schema_migrations (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'failed', $3)", migration.Version, migration.Direction, dirty)
	return multierror.Append(fmt.Errorf("Migration '%s' failed: %v", migration.Name, err), dbErr)
}

func (m *migrator) runMigration(migration migration) error {
	var err error

	switch migration.Strategy {
	case GoMigration:
		err = migrations.NewMigrations(m.db, m.strategy).Run(migration.Name)
		if err != nil {
			return m.recordMigrationFailure(migration, err, false)
		}
	case SQLTransaction:
		tx, err := m.db.Begin()
		for _, statement := range migration.Statements {
			_, err = tx.Exec(statement)
			if err != nil {
				tx.Rollback()
				err = multierror.Append(fmt.Errorf("Transaction %v failed, rolled back the migration", statement), err)
				if err != nil {
					return m.recordMigrationFailure(migration, err, false)
				}
			}
		}
		err = tx.Commit()
	case SQLNoTransaction:
		_, err = m.db.Exec(migration.Statements[0])
		if err != nil {
			return m.recordMigrationFailure(migration, err, true)
		}
	}

	_, err = m.db.Exec("INSERT INTO schema_migrations (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, $2, 'passed', false)", migration.Version, migration.Direction)
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
	sort.Slice(migrationList, func(i, j int) bool { return migrationList[i].Version < migrationList[j].Version })

	return migrationList, nil
}

func (self *migrator) Up() error {
	migrations, err := self.Migrations()
	if err != nil {
		return err
	}
	return self.Migrate(migrations[len(migrations)-1].Version)
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

func (self *migrator) existLegacyVersion() bool {
	var exists bool
	err := self.db.QueryRow("SELECT EXISTS ( SELECT 1 FROM information_schema.tables WHERE table_name = 'migration_version')").Scan(&exists)
	return err != nil || exists
}

func (self *migrator) convertLegacySchemaTableToCurrent() error {
	oldMigrationLastVersion := 189
	newMigrationStartVersion := 1510262030

	var err error
	var dbVersion int

	exists := self.existLegacyVersion()
	if !exists {
		return nil
	}

	if err = self.db.QueryRow("SELECT version FROM migration_version").Scan(&dbVersion); err != nil {
		return err
	}

	if dbVersion != oldMigrationLastVersion {
		return fmt.Errorf("Must upgrade from db version %d (concourse 3.6.0), current db version: %d", oldMigrationLastVersion, dbVersion)
	}

	if _, err = self.db.Exec("DROP TABLE IF EXISTS migration_version"); err != nil {
		return err
	}

	_, err = self.db.Exec("INSERT INTO schema_migrations (version, tstamp, direction, status, dirty) VALUES ($1, current_timestamp, 'up', 'passed', false)", newMigrationStartVersion)
	if err != nil {
		return err
	}

	return nil
}

type filenames []string

func (m filenames) Len() int {
	return len(m)
}

func (m filenames) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m filenames) Less(i, j int) bool {
	m1, _ := source.Parse(m[i])
	m2, _ := source.Parse(m[j])
	return m1.Version < m2.Version
}

func (m filenames) Latest() string {
	matches := []string{}

	for _, match := range m {
		if _, err := source.Parse(match); err == nil {
			matches = append(matches, match)
		}
	}

	sort.Sort(filenames(matches))

	return matches[len(matches)-1]
}
