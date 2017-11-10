package migration

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/mattes/migrate/database"
	"github.com/mattes/migrate/source"
	_ "github.com/mattes/migrate/source/file"

	"github.com/mattes/migrate"
	"github.com/mattes/migrate/database/postgres"
	"github.com/mattes/migrate/source/go-bindata"
)

func Open(driver, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	d, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		db.Close()
		return nil, err
	}

	s, err := bindata.WithInstance(bindata.Resource(
		AssetNames(),
		func(name string) ([]byte, error) {
			return Asset(name)
		}),
	)

	dbConn, err := OpenWithMigrateDrivers(db, "go-bindata", s, "postgres", d)
	if err != nil {
		db.Close()
		return nil, err
	}

	return dbConn, nil
}

func OpenWithMigrateDrivers(db *sql.DB, sourceName string, s source.Driver, databaseName string, d database.Driver) (*sql.DB, error) {
	m, err := migrate.NewWithInstance(sourceName, s, databaseName, d)
	if err != nil {
		return nil, err
	}

	forceVersion, err := checkMigrationVersion(db)

	if err != nil {
		return nil, err
	}

	if forceVersion > 0 {
		if err = m.Force(forceVersion); err != nil {
			return nil, err
		}
	}

	if err = m.Up(); err != nil {
		if err.Error() != "no change" {
			return nil, err
		}
	}

	return db, nil
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

	if _, err = db.Exec("DROP TABLE migration_version"); err != nil {
		return -1, nil
	}

	if dbVersion == oldMigrationLastVersion {
		return newMigrationStartVersion, nil
	}

	return -1, fmt.Errorf("Unkown database version: %d", dbVersion)
}
