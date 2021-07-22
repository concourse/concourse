package db

//counterfeiter:generate . CheckLifecycle
type CheckLifecycle interface {
	DeleteCompletedChecks() error
}

type checkLifecycle struct {
	conn Conn
}

func NewCheckLifecycle(conn Conn) CheckLifecycle {
	return &checkLifecycle{
		conn: conn,
	}
}

func (cl *checkLifecycle) DeleteCompletedChecks() error {
	_, err1 := cl.conn.Exec(`
      WITH resource_builds AS (
        SELECT distinct(last_check_build_id) as build_id
        FROM resource_config_scopes
        WHERE last_check_build_id IS NOT NULL
      ),
      deleted_builds AS (
        DELETE FROM builds USING (
          SELECT id
          FROM builds b
          WHERE completed AND resource_id IS NOT NULL
          AND NOT EXISTS ( SELECT 1 FROM resource_builds WHERE build_id = b.id )
            UNION ALL
          SELECT id
          FROM builds b
          WHERE completed AND resource_type_id IS NOT NULL
          AND EXISTS (SELECT * FROM builds b2 WHERE b.resource_type_id = b2.resource_type_id AND b.id < b2.id)
		) AS deletable_builds WHERE builds.id = deletable_builds.id
		RETURNING builds.id
      )
      DELETE FROM check_build_events USING deleted_builds WHERE build_id = deleted_builds.id
    `)

	_, err2 := cl.conn.Exec(`
      WITH resource_builds AS (
        SELECT distinct(last_check_build_id) as build_id
        FROM resource_config_scopes
        WHERE last_check_build_id IS NOT NULL
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
