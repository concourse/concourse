package db

import (
	"fmt"

	"github.com/concourse/atc/db/lock"
)

type SQLDB struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewSQL(
	sqldbConnection Conn,
	lockFactory lock.LockFactory,
) *SQLDB {
	return &SQLDB{
		conn:        sqldbConnection,
		lockFactory: lockFactory,
	}
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}

type scannable interface {
	Scan(destinations ...interface{}) error
}
