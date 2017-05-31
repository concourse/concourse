package metric

import (
	"database/sql"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/atc/dbng"
)

func CountQueries(conn dbng.Conn) dbng.Conn {
	return &countingConn{
		Conn: conn,
	}
}

type countingConn struct {
	dbng.Conn
}

func (e *countingConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	DatabaseQueries.Inc()

	return e.Conn.Query(query, args...)
}

func (e *countingConn) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	DatabaseQueries.Inc()

	return e.Conn.QueryRow(query, args...)
}

func (e *countingConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	DatabaseQueries.Inc()

	return e.Conn.Exec(query, args...)
}

func (e *countingConn) Begin() (dbng.Tx, error) {
	tx, err := e.Conn.Begin()
	if err != nil {
		return tx, err
	}

	return &countingTx{Tx: tx}, nil
}

type countingTx struct {
	dbng.Tx
}

func (e *countingTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	DatabaseQueries.Inc()

	return e.Tx.Query(query, args...)
}

func (e *countingTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	DatabaseQueries.Inc()

	return e.Tx.QueryRow(query, args...)
}

func (e *countingTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	DatabaseQueries.Inc()

	return e.Tx.Exec(query, args...)
}
