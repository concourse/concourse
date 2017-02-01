package migrations

import (
	"database/sql"
	"hash/crc32"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/dbng"

	"github.com/concourse/atc/dbng/migration"
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
		dbConn, err = db.WrapWithError(migration.Open(sqlDriver, sqlDataSource, migrations))
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

func DBNGConn(logger lager.Logger, sqlDriver string, sqlDataSource string) (dbng.Conn, error) {
	var err error
	var dbConn dbng.Conn

	for {
		dbConn, err = dbng.WrapWithError(sql.Open(sqlDriver, sqlDataSource))
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

	return dbConn, nil
}
