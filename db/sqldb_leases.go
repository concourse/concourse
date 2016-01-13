package db

import (
	"database/sql"
	"time"

	"github.com/pivotal-golang/lager"
)

func (db *SQLDB) LeaseBuildTracking(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
					AND now() - last_tracked > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
			`, buildID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (db *SQLDB) LeaseBuildScheduling(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
					AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
			`, buildID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (db *SQLDB) LeaseCacheInvalidation(interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"CacheInvalidator": "Scottsboro",
		}),
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			_, err := tx.Exec(`
				INSERT INTO cache_invalidator (last_invalidated)
				SELECT 'epoch'
				WHERE NOT EXISTS (SELECT * FROM cache_invalidator)`)
			if err != nil {
				return nil, err
			}
			return tx.Exec(`
				UPDATE cache_invalidator
				SET last_invalidated = now()
				WHERE now() - last_invalidated > ($1 || ' SECONDS')::INTERVAL
			`, interval.Seconds())
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE cache_invalidator
				SET last_invalidated = now()
			`)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}
