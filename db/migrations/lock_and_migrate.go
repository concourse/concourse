package migrations

import (
	"database/sql"
	"hash/crc32"
	"strings"
	"time"

	"github.com/concourse/atc/db"
	"code.cloudfoundry.org/lager"

	"github.com/BurntSushi/migration"
)

func LockDBAndMigrate(logger lager.Logger, sqlDriver string, sqlDataSource string) (db.Conn, error) {
	var err error
	var dbLockConn db.Conn
	var dbConn db.Conn

	for {
		dbLockConn, err = db.WrapWithError(sql.Open(sqlDriver, sqlDataSource))
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, err
		}

		break
	}

	lockName := crc32.ChecksumIEEE([]byte(sqlDriver + sqlDataSource))

	for {
		_, err = dbLockConn.Exec(`select pg_advisory_lock($1)`, lockName)
		if err != nil {
			logger.Error("failed-to-acquire-lock-retrying", err)
			time.Sleep(5 * time.Second)
			continue
		}

		logger.Info("migration-lock-acquired")

		migrations := Translogrifier(logger, Migrations)
		dbConn, err = db.WrapWithError(migration.OpenWith(sqlDriver, sqlDataSource, migrations, safeGetVersion, safeSetVersion))
		if err != nil {
			logger.Fatal("failed-to-run-migrations", err)
		}

		_, err = dbLockConn.Exec(`select pg_advisory_unlock($1)`, lockName)
		if err != nil {
			logger.Error("failed-to-release-lock", err)
		}

		dbLockConn.Close()
		break
	}

	return dbConn, nil
}

func safeGetVersion(tx migration.LimitedTx) (int, error) {
	v, err := getVersion(tx)
	if err != nil {
		if err := createVersionTable(tx); err != nil {
			return 0, err
		}
		return getVersion(tx)
	}
	return v, nil
}

func safeSetVersion(tx migration.LimitedTx, version int) error {
	if err := setVersion(tx, version); err != nil {
		if err := createVersionTable(tx); err != nil {
			return err
		}
		return setVersion(tx, version)
	}
	return nil
}

func getVersion(tx migration.LimitedTx) (int, error) {
	var version int
	r := tx.QueryRow("SELECT version FROM migration_version")
	if err := r.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func setVersion(tx migration.LimitedTx, version int) error {
	_, err := tx.Exec("UPDATE migration_version SET version = $1", version)
	return err
}

func createVersionTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
		CREATE TABLE migration_version (
			version INTEGER
		)
	`)
	if err != nil {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO migration_version (version) VALUES (0)
	`)
	return err
}
