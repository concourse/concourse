package db

import (
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . CheckLifecycle

type CheckLifecycle interface {
	RemoveExpiredChecks(time.Duration) error
}

type checkLifecycle struct {
	conn Conn
}

func NewCheckLifecycle(conn Conn) *checkLifecycle {
	return &checkLifecycle{
		conn: conn,
	}
}

func (lifecycle *checkLifecycle) RemoveExpiredChecks(recyclePeriod time.Duration) error {

	_, err := psql.Delete("checks").
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

	return err
}
