package db

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . CheckLifecycle

type CheckLifecycle interface {
	RemoveExpiredChecks(time.Duration) (int, error)
}

type checkLifecycle struct {
	conn Conn
}

func NewCheckLifecycle(conn Conn) *checkLifecycle {
	return &checkLifecycle{
		conn: conn,
	}
}

func (lifecycle *checkLifecycle) RemoveExpiredChecks(recyclePeriod time.Duration) (int, error) {

	result, err := psql.Delete("checks").
		Where(
			sq.And{
				sq.Gt{
					"Now() - create_time": fmt.Sprintf("%.0f seconds", recyclePeriod.Seconds()),
				},
				sq.NotEq{"status": CheckStatusStarted},
			},
		).
		RunWith(lifecycle.conn).
		Exec()

	if err != nil {
		return 0, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(affected), nil
}
