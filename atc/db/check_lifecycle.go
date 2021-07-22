package db

import "code.cloudfoundry.org/lager"

var CheckDeleteBatchSize = 500

//counterfeiter:generate . CheckLifecycle
type CheckLifecycle interface {
	DeleteCompletedChecks(logger lager.Logger) error
}

type checkLifecycle struct {
	conn Conn
}

func NewCheckLifecycle(conn Conn) CheckLifecycle {
	return &checkLifecycle{
		conn: conn,
	}
}

func (cl *checkLifecycle) DeleteCompletedChecks(logger lager.Logger) error {
	var counter int
	for {
		var numChecksDeleted int
		err := cl.conn.QueryRow(`
      WITH resource_builds AS (
        SELECT build_id
        FROM resources
        WHERE build_id IS NOT NULL
      ),
      deleted_builds AS (
        DELETE FROM builds USING (
          (SELECT id
          FROM builds b
          WHERE completed AND resource_id IS NOT NULL
          AND NOT EXISTS ( SELECT 1 FROM resource_builds WHERE build_id = b.id )
					LIMIT $1)
            UNION ALL
          SELECT id
          FROM builds b
          WHERE completed AND resource_type_id IS NOT NULL
          AND EXISTS (SELECT * FROM builds b2 WHERE b.resource_type_id = b2.resource_type_id AND b.id < b2.id)
    ) AS deletable_builds WHERE builds.id = deletable_builds.id
      RETURNING builds.id
      ), deleted_events AS (
        DELETE FROM check_build_events USING deleted_builds WHERE build_id = deleted_builds.id
      )
      SELECT COUNT(*) FROM deleted_builds
    `, CheckDeleteBatchSize).Scan(&numChecksDeleted)
		if err != nil {
			return err
		}
		logger.Debug("deleted-check-builds", lager.Data{"count": numChecksDeleted, "batch": counter})

		if numChecksDeleted < CheckDeleteBatchSize {
			break
		}
		counter++
	}

	return nil
}
