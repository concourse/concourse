package db

import (
	"database/sql"
	"net"
	"reflect"
)

type RetryableConn struct {
	Connector Connector

	conn DelegateConn // *sql.DB
}

//go:generate counterfeiter . DelegateConn

type DelegateConn interface {
	Query(sql string, args ...interface{}) (*sql.Rows, error)
	Exec(sql string, arguments ...interface{}) (sql.Result, error)
	Close() error
}

//go:generate counterfeiter . Connector

type Connector interface {
	Connect() (DelegateConn, error)
}

type SQLConnector struct {
	SQLConfig string
}

func (c SQLConnector) Connect() (DelegateConn, error) {
	db, err := sql.Open("postgres", c.SQLConfig)
	if err != nil {
		return nil, err
	}

	db.SetMaxIdleConns(1)
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	return db, nil
}

func (c *RetryableConn) Exec(sql string, arguments ...interface{}) (sql.Result, error) {
	err := c.connect()
	if err != nil {
		return nil, err
	}

	return c.conn.Exec(sql, arguments...)
}

func (c *RetryableConn) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	err := c.connect()
	if err != nil {
		return nil, err
	}

	rows, queryErr := c.conn.Query(sql, args...)
	if queryErr != nil {
		var connError *net.OpError

		for queryErr != nil && reflect.TypeOf(queryErr) == reflect.TypeOf(connError) {
			err := c.reconnect()
			if err != nil {
				continue
			}

			rows, queryErr = c.conn.Query(sql, args...)
		}
	}

	return rows, queryErr
}

func (c *RetryableConn) reconnect() error {
	conn, err := c.Connector.Connect()
	if err != nil {
		return err
	}

	c.conn = conn

	return nil
}

func (c *RetryableConn) connect() error {
	if c.conn != nil {
		return nil
	}

	return c.reconnect()
}
