package migration

import (
	"database/sql"
	"fmt"
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

func Open(driver, dsn string) (*sql.DB, error) {
	return OpenWithLockFactory(driver, dsn, nil)
}

func OpenWithLockFactory(driver, dsn string, lockFactory lock.LockFactory) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	s, err := bindata.WithInstance(bindata.Resource(
		AssetNames(),
		func(name string) ([]byte, error) {
			return Asset(name)
		}),
	)

	dbConn, err := OpenWithMigrateDrivers(db, "go-bindata", s, lockFactory)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return dbConn, nil
}

func OpenWithMigrateDrivers(db *sql.DB, sourceName string, s source.Driver, lockFactory lock.LockFactory) (*sql.DB, error) {

	logger := lager.NewLogger("migrations").Session("locks")
	for {

		if lockFactory != nil {
			lock, acquired, err := lockFactory.Acquire(logger, lock.NewDatabaseMigrationLockID())
			if err != nil {
				return nil, err
			}

			if !acquired {
				time.Sleep(1 * time.Second)
				continue
			}

			defer lock.Release()
		}

		forceVersion, err := checkMigrationVersion(db)

		if err != nil {
			return nil, err
		}

		d, err := postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			_ = db.Close()
			return nil, err
		}

		m, err := migrate.NewWithInstance(sourceName, s, "postgres", d)
		if err != nil {
			return nil, err
		}

		if forceVersion > 0 {
			if err = m.Force(forceVersion); err != nil {
				logger.Error("migrations", err)
				return nil, err
			}
		}

		if err = m.Up(); err != nil {
			if err.Error() != "no change" {
				logger.Error("migrations", err)
				return nil, err
			}
		}

		return db, nil
	}
}

func checkMigrationVersion(db *sql.DB) (int, error) {
	oldMigrationLastVersion := 189
	newMigrationStartVersion := 1510262030

	var err error
	var dbVersion int

	if err = db.QueryRow("SELECT version FROM migration_version").Scan(&dbVersion); err != nil {
		return -1, nil
	}

	if dbVersion < oldMigrationLastVersion {
		return -1, fmt.Errorf("Cannot upgrade from concourse version < 3.6.0 (db version: %d), current db version: %d", oldMigrationLastVersion, dbVersion)
	}

	if _, err = db.Exec("DROP TABLE IF EXISTS migration_version"); err != nil {
		return -1, nil
	}

	if dbVersion == oldMigrationLastVersion {
		return newMigrationStartVersion, nil
	}

	return -1, fmt.Errorf("Unkown database version: %d", dbVersion)
}
