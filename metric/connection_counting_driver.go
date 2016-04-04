package metric

import (
	"database/sql"
	"database/sql/driver"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Prepare(query string) (driver.Stmt, error)
	Close() error
	Begin() (driver.Tx, error)
}

//go:generate counterfeiter . Driver

type Driver interface {
	Open(name string) (driver.Conn, error)
}

type connectionCountingDriver struct {
	Driver
}

func SetupConnectionCountingDriver(delegateDriverName, sqlDataSource, newDriverName string) {
	delegateDBConn, err := sql.Open(delegateDriverName, sqlDataSource)
	if err == nil {
		// ignoring any connection errors since we only need this to access the driver struct
		delegateDBConn.Close()
	}

	connectionCountingDriver := &connectionCountingDriver{delegateDBConn.Driver()}
	sql.Register(newDriverName, connectionCountingDriver)
}

func (d *connectionCountingDriver) Open(name string) (driver.Conn, error) {
	delegateConn, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}

	DatabaseConnections.Inc()
	return &connectionCountingConn{delegateConn}, nil
}

type connectionCountingConn struct {
	driver.Conn
}

func (c *connectionCountingConn) Close() error {
	err := c.Conn.Close()
	if err != nil {
		return err
	}

	DatabaseConnections.Dec()
	return nil
}
