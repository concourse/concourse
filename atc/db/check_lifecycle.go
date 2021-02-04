package db

//go:generate counterfeiter . CheckLifecycle

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
	_, err := cl.conn.Exec(`
      WITH deletable_builds AS
      (
        SELECT id
        FROM builds b
        WHERE completed AND resource_id IS NOT NULL
        AND EXISTS (SELECT * FROM builds b2 WHERE b.resource_id = b2.resource_id AND b.id < b2.id)
          UNION ALL
        SELECT id
        FROM builds b
        WHERE completed AND resource_type_id IS NOT NULL
        AND EXISTS (SELECT * FROM builds b2 WHERE b.resource_type_id = b2.resource_type_id AND b.id < b2.id)
      ),
      deleted_builds AS
      (
        DELETE FROM builds WHERE id IN (SELECT id FROM deletable_builds)
        RETURNING id
      )
      DELETE FROM check_build_events WHERE build_id IN (SELECT id FROM deletable_builds)
    `)
	return err
}
