package dbng

import (
	"database/sql"
	"database/sql/driver"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/dbng/migration"
	"github.com/lib/pq"
)

type Conn interface {
	Bus() NotificationsBus

	Begin() (Tx, error)
	Close() error
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	Ping() error
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
}

type Tx interface {
	Commit() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner
	Rollback() error
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func Open(logger lager.Logger, sqlDriver string, sqlDataSource string) (Conn, error) {
	for {
		sqlDb, err := migration.Open(sqlDriver, sqlDataSource, migrations.Migrations)
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, err
		}

		listener := pq.NewListener(sqlDataSource, time.Second, time.Minute, nil)

		return &db{
			DB: sqlDb,

			bus: NewNotificationsBus(listener, sqlDb),
		}, nil
	}
}

type db struct {
	*sql.DB

	bus NotificationsBus
}

func (db *db) Bus() NotificationsBus {
	return db.bus
}

func (db *db) Begin() (Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &dbTx{tx}, nil
}

// to conform to squirrel.Runner interface
func (db *db) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return db.DB.QueryRow(query, args...)
}

type dbTx struct {
	*sql.Tx
}

// to conform to squirrel.Runner interface
func (tx *dbTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return tx.Tx.QueryRow(query, args...)
}
