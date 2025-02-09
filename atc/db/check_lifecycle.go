package db

import "code.cloudfoundry.org/lager/v3"

var CheckDeleteBatchSize = 500

//counterfeiter:generate . CheckLifecycle
type CheckLifecycle interface {
	DeleteCompletedChecks(logger lager.Logger) error
}

type checkLifecycle struct {
	conn DbConn
}

func NewCheckLifecycle(conn DbConn) CheckLifecycle {
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
      WITH expired_imb_ids AS (
          SELECT distinct(build_id) AS build_id
              FROM check_build_events cbe
              WHERE NOT EXISTS (SELECT 1 FROM resources WHERE in_memory_build_id = cbe.build_id)
                AND NOT EXISTS (SELECT 1 FROM builds WHERE id = cbe.build_id)
      )
      DELETE FROM check_build_events cbe2 USING expired_imb_ids 
          WHERE cbe2.build_id = expired_imb_ids.build_id;
    `)

	if err1 != nil {
		return err1
	}

	if err2 != nil {
		return err2
	}

	return nil
}
