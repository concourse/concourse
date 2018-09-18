package db

import (
	"database/sql"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/Masterminds/squirrel"
)

func Log(logger lager.Logger, conn Conn) Conn {
	return &logConn{
		Conn:   conn,
		logger: logger,
	}
}

type logConn struct {
	Conn

	logger lager.Logger
}

func (c *logConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	c.logger.Debug("query", lager.Data{"query": c.strip(query)})
	return c.Conn.Query(query, args...)
}

func (c *logConn) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	c.logger.Debug("query-row", lager.Data{"query": c.strip(query)})
	return c.Conn.QueryRow(query, args...)
}

func (c *logConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	c.logger.Debug("exec", lager.Data{"query": c.strip(query)})
	return c.Conn.Exec(query, args...)
}

func (c *logConn) strip(query string) string {
	return strings.Join(strings.Fields(query), " ")
}
