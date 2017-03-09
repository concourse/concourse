package dbng

import (
	"database/sql"
	"database/sql/driver"

	"github.com/Masterminds/squirrel"
)

type Conn interface {
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
	tx, err := wrapped.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &wrappedTx{tx}, nil
}

// to conform to squirrel.Runner interface
func (wrapped *wrappedDB) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return wrapped.DB.QueryRow(query, args...)
}

type wrappedTx struct {
	*sql.Tx
}

// to conform to squirrel.Runner interface
func (wrapped *wrappedTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return wrapped.Tx.QueryRow(query, args...)
}
