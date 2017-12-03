package migration

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
	"github.com/mattes/migrate"
	"github.com/mattes/migrate/database/postgres"
	"github.com/mattes/migrate/source"
	"github.com/mattes/migrate/source/go-bindata"

	_ "github.com/lib/pq"
	_ "github.com/mattes/migrate/source/file"
)

func NewOpenHelper(driver, name string, lockFactory lock.LockFactory) *OpenHelper {
	return &OpenHelper{
		driver,
		name,
		lockFactory,
	}
}

type OpenHelper struct {
	driver         string
	dataSourceName string
	lockFactory    lock.LockFactory
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

	if err := NewMigrator(db, self.lockFactory).Up(); err != nil {
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

	if err := NewMigrator(db, self.lockFactory).Migrate(version); err != nil {
		return err
	}

	return nil
}

type Migrator interface {
	CurrentVersion() (int, error)
	SupportedVersion() (int, error)
	Migrate(version int) error
	Up() error
}

func NewMigrator(db *sql.DB, lockFactory lock.LockFactory) Migrator {
	return NewMigratorForMigrations(db, lockFactory, AssetNames())
}

func NewMigratorForMigrations(db *sql.DB, lockFactory lock.LockFactory, migrations []string) Migrator {
	return &migrator{
		db,
		lockFactory,
		lager.NewLogger("migrations"),
		migrations,
	}
}

type migrator struct {
	db          *sql.DB
	lockFactory lock.LockFactory
	logger      lager.Logger
	migrations  migrations
}

func (self *migrator) SupportedVersion() (int, error) {

	latest := self.migrations.Latest()

	m, err := source.Parse(latest)
	if err != nil {
		return -1, err
	}

	return int(m.Version), nil
}

func (self *migrator) CurrentVersion() (int, error) {
	m, lock, err := self.openWithLock()
	if err != nil {
		return -1, err
	}

	if lock != nil {
		defer lock.Release()
	}

	version, _, err := m.Version()
	if err != nil {
		return -1, err
	}

	return int(version), nil
}

func (self *migrator) Migrate(version int) error {

	m, lock, err := self.openWithLock()
	if err != nil {
		return err
	}

	if lock != nil {
		defer lock.Release()
	}

	if err = m.Migrate(uint(version)); err != nil {
		if err.Error() != "no change" {
			return err
		}
	}

	return nil
}

func (self *migrator) Up() error {

	m, lock, err := self.openWithLock()
	if err != nil {
		return err
	}

	if lock != nil {
		defer lock.Release()
	}

	if err = m.Up(); err != nil {
		if err.Error() != "no change" {
			return err
		}
	}

	return nil
}

func (self *migrator) open() (*migrate.Migrate, error) {

	forceVersion, err := self.checkLegacyVersion()
	if err != nil {
		return nil, err
	}

	s, err := bindata.WithInstance(bindata.Resource(
		self.migrations,
		func(name string) ([]byte, error) {
			return Asset(name)
		}),
	)

	d, err := postgres.WithInstance(self.db, &postgres.Config{})
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithInstance("go-bindata", s, "postgres", d)
	if err != nil {
		return nil, err
	}

	if forceVersion > 0 {
		if err = m.Force(forceVersion); err != nil {
			return nil, err
		}
	}

	return m, nil
}

func (self *migrator) openWithLock() (*migrate.Migrate, lock.Lock, error) {

	var err error
	var acquired bool
	var newLock lock.Lock

	if self.lockFactory != nil {
		for {
			newLock, acquired, err = self.lockFactory.Acquire(self.logger, lock.NewDatabaseMigrationLockID())

			if err != nil {
				return nil, nil, err
			}

			if acquired {
				break
			}

			time.Sleep(1 * time.Second)
		}
	}

	m, err := self.open()

	if err != nil && newLock != nil {
		newLock.Release()
		return nil, nil, err
	}

	return m, newLock, err
}

func (self *migrator) checkLegacyVersion() (int, error) {
	oldMigrationLastVersion := 189
	newMigrationStartVersion := 1510262030

	var err error
	var dbVersion int

	if err = self.db.QueryRow("SELECT version FROM migration_version").Scan(&dbVersion); err != nil {
		return -1, nil
	}

	if dbVersion != oldMigrationLastVersion {
		return -1, fmt.Errorf("Must upgrade from db version %d (concourse 3.6.0), current db version: %d", oldMigrationLastVersion, dbVersion)
	}

	if _, err = self.db.Exec("DROP TABLE IF EXISTS migration_version"); err != nil {
		return -1, err
	}

	return newMigrationStartVersion, nil
}

type migrations []string

func (m migrations) Len() int {
	return len(m)
}

func (m migrations) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func (m migrations) Less(i, j int) bool {
	m1, _ := source.Parse(m[i])
	m2, _ := source.Parse(m[j])
	return m1.Version < m2.Version
}

func (m migrations) Latest() string {
	sort.Sort(m)

	return m[len(m)-1]
}
