package db

import (
	"database/sql"
	"database/sql/driver"
	"errors"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Begin() (Tx, error)
	Close() error
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	Ping() error
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
}

//go:generate counterfeiter . Tx

type Tx interface {
	Commit() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Rollback() error
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func Wrap(sqlDB *sql.DB) Conn {
	return &wrappedDB{DB: sqlDB}
}

func WrapWithError(sqlDB *sql.DB, err error) (Conn, error) {
	return &wrappedDB{DB: sqlDB}, err
}

type wrappedDB struct {
	*sql.DB
}

func (wrapped *wrappedDB) Begin() (Tx, error) {
	return wrapped.DB.Begin()
}

func swallowUniqueViolation(err error) error {
	if err != nil {
		if pgErr, ok := err.(*pq.Error); ok {
			if pgErr.Code.Class().Name() == "integrity_constraint_violation" {
				return nil
			}
		}

		return err
	}

	return nil
}

type DB interface {
	CreateTeam(team Team) (SavedTeam, error)
	CreateDefaultTeamIfNotExists() error

	CreatePipe(pipeGUID string, url string, teamName string) error
	GetPipe(pipeGUID string) (Pipe, error)

	GetTaskLock(logger lager.Logger, taskName string) (lock.Lock, bool, error)
}

//go:generate counterfeiter . Notifier

type Notifier interface {
	Notify() <-chan struct{}
	Close() error
}

//ConfigVersion is a sequence identifier used for compare-and-swap
type ConfigVersion int

var ErrConfigComparisonFailed = errors.New("comparison with existing config failed during save")
var ErrEndOfBuildEventStream = errors.New("end of build event stream")
var ErrBuildEventStreamClosed = errors.New("build event stream closed")

//go:generate counterfeiter . EventSource

type EventSource interface {
	Next() (event.Envelope, error)
	Close() error
}
