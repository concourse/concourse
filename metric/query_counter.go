package metric

import (
	"database/sql"

	"github.com/concourse/atc/db"
)

func CountQueries(conn db.Conn) db.Conn {
	return &countingConn{
		Conn: conn,
	}
}

type countingConn struct {
	db.Conn
}

func (e *countingConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	DatabaseQueries.Inc()

	return e.Conn.Query(query, args...)
}

func (e *countingConn) QueryRow(query string, args ...interface{}) *sql.Row {
	DatabaseQueries.Inc()

	return e.Conn.QueryRow(query, args...)
}

func (e *countingConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	DatabaseQueries.Inc()

	return e.Conn.Exec(query, args...)
}
