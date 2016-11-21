package lock

import (
	"net"
	"reflect"

	"github.com/jackc/pgx"
)

type RetryableConn struct {
	Connector Connector
	Conn      DelegateConn // *pgx.Conn
}

//go:generate counterfeiter . DelegateConn

type DelegateConn interface {
	Query(sql string, args ...interface{}) (*pgx.Rows, error)
	QueryRow(sql string, args ...interface{}) *pgx.Row
	Exec(sql string, arguments ...interface{}) (commandTag pgx.CommandTag, err error)
}

//go:generate counterfeiter . Connector

type Connector interface {
	Connect() (DelegateConn, error)
}

type PgxConnector struct {
	PgxConfig pgx.ConnConfig
}

func (c PgxConnector) Connect() (DelegateConn, error) {
	return pgx.Connect(c.PgxConfig)
}

func (c *RetryableConn) Exec(sql string, arguments ...interface{}) (pgx.CommandTag, error) {
	return c.Conn.Exec(sql, arguments...)
}

func (c *RetryableConn) QueryRow(sql string, args ...interface{}) *pgx.Row {
	rows, queryErr := c.Conn.Query(sql, args...)
	if queryErr != nil {
		var connError *net.OpError

		for queryErr != nil && reflect.TypeOf(queryErr) == reflect.TypeOf(connError) {
			err := c.reconnect()
			if err != nil {
				continue
			}
			rows, queryErr = c.Conn.Query(sql, args...)
		}
	}

	return (*pgx.Row)(rows)
}

func (c *RetryableConn) reconnect() error {
	deleteConn, err := c.Connector.Connect()
	if err != nil {
		return err
	}

	c.Conn = deleteConn
	return nil
}
