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
	var err1 error
	for {
		var numChecksDeleted int
		err1 = cl.conn.QueryRow(`
      WITH resource_builds AS (
        SELECT distinct(last_check_build_id) as build_id
        FROM resource_config_scopes
        WHERE last_check_build_id IS NOT NULL
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
		if err1 != nil {
			break
		}
		logger.Debug("deleted-check-builds", lager.Data{"count": numChecksDeleted, "batch": counter})

		if numChecksDeleted < CheckDeleteBatchSize {
			break
		}
		counter++
	}

	_, err2 := cl.conn.Exec(`
      WITH resource_builds AS (
        SELECT distinct(last_check_build_id) as build_id
        FROM resource_config_scopes
        WHERE last_check_build_id IS NOT NULL
		  UNION ALL
        SELECT distinct(in_memory_build_id) as build_id
        FROM resources
        WHERE in_memory_build_id IS NOT NULL
      )
      DELETE FROM check_build_events WHERE build_id NOT IN (SELECT build_id FROM resource_builds)
    `)

	if err1 != nil {
		return err1
	}

	if err2 != nil {
		return err2
	}

	return nil
}
