package db

import (
	"database/sql"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db/lock"
)

type SuccessfulBuildOutputsMigrator struct {
	batchSize int

	dbConn      Conn
	lockFactory lock.LockFactory
}

func NewSuccessfulBuildOutputsMigrator(dbConn Conn, lockFactory lock.LockFactory, batchSize int) *SuccessfulBuildOutputsMigrator {
	return &SuccessfulBuildOutputsMigrator{
		batchSize: batchSize,

		dbConn:      dbConn,
		lockFactory: lockFactory,
	}
}

func (s *SuccessfulBuildOutputsMigrator) AcquireMigrationLock(
	logger lager.Logger,
) (lock.Lock, bool, error) {
	return s.lockFactory.Acquire(
		logger,
		lock.NewSuccessfulBuildOutputsLockID(),
	)
}

func (s *SuccessfulBuildOutputsMigrator) Migrate(logger lager.Logger) error {
	// this selects the value and if it's 0 or ErrNoRows just exits
	// otherwise, select with builds < value and > value-(batch size)
	// after migrating batch, update value to max(value-(batch size), 0)
	// do that in a loop
	logger.Info("starting-successful-build-outputs-migrator")

	var buildIDCursor int
	err := psql.Select("build_id_cursor").
		From("successful_build_outputs_migrator").
		RunWith(s.dbConn).
		QueryRow().
		Scan(&buildIDCursor)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	for buildIDCursor > 0 {

		nextCursor := buildIDCursor - s.batchSize
		if nextCursor < 0 {
			nextCursor = 0
		}

		tx, err := s.dbConn.Begin()
		if err != nil {
			return err
		}

		defer Rollback(tx)

		_, err = tx.Exec(`
	INSERT INTO successful_build_outputs (
		SELECT b.id, b.job_id, json_object_agg(sp.resource_id, sp.v) FROM builds b
		LEFT JOIN (
			SELECT build_id, resource_id, json_agg(version_md5) as v FROM (
				(
					SELECT build_id, resource_id, version_md5 FROM build_resource_config_version_outputs o WHERE build_id <= $1 AND build_id > $2
				)
				UNION ALL
				(
					SELECT build_id, resource_id, version_md5 FROM build_resource_config_version_inputs i WHERE build_id <= $1 AND build_id > $2
				)
			) AS agg GROUP BY build_id, resource_id
		) sp ON sp.build_id = b.id
	WHERE status = 'succeeded' AND id <= $1 AND id > $2 GROUP BY b.id, b.job_id
);`, buildIDCursor, nextCursor)
		if err != nil {
			return err
		}

		logger.Info("migrated-builds", lager.Data{"from": buildIDCursor, "to": nextCursor})

		buildIDCursor = nextCursor

		_, err = psql.Update("successful_build_outputs_migrator").
			Set("build_id_cursor", buildIDCursor).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}
	}

	_, err = s.dbConn.Exec("DROP TABLE successful_build_outputs_migrator")
	if err != nil {
		return err
	}

	logger.Info("deleted-temp-migrator-table")

	return nil
}
