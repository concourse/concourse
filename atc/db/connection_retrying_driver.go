package db

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/lib/pq"
)

type connectionRetryingDriver struct {
	driver.Driver
	driverName        string
	keepAliveIdleTime time.Duration
	keepAliveCount    int
	keepAliveInterval time.Duration
}

func SetupConnectionRetryingDriver(
	delegateDriverName string,
	sqlDataSource string,
	newDriverName string,
	keepAliveIdleTime time.Duration,
	keepAliveCount int,
	keepAliveInterval time.Duration,
) {
	for _, driverName := range sql.Drivers() {
		if driverName == newDriverName {
			return
		}
	}
	delegateDBConn, err := sql.Open(delegateDriverName, sqlDataSource)
	if err == nil {
		// ignoring any connection errors since we only need this to access the driver struct
		_ = delegateDBConn.Close()
	}

	connectionRetryingDriver := &connectionRetryingDriver{
		delegateDBConn.Driver(),
		delegateDriverName,
		keepAliveIdleTime,
		keepAliveCount,
		keepAliveInterval,
	}
	sql.Register(newDriverName, connectionRetryingDriver)
}

func (d *connectionRetryingDriver) Open(name string) (driver.Conn, error) {
	var conn driver.Conn

	err := backoff.Retry(func() error {
		var err error
		if d.driverName == "postgres" {
			conn, err = pq.DialOpen(&keepAliveDialer{
				keepAliveIdleTime: d.keepAliveIdleTime,
				keepAliveCount:    d.keepAliveCount,
				keepAliveInterval: d.keepAliveInterval,
			}, name)
		} else {
			conn, err = d.Driver.Open(name)
		}
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == "too_many_connections" {
				return err
			}

			return backoff.Permanent(err)
		}

		return nil
	}, backoff.NewExponentialBackOff())
	if err != nil {
		return nil, err
	}

	return conn, nil
}
