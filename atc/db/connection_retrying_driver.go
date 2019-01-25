package db

import (
	"database/sql"
	"database/sql/driver"
	"github.com/concourse/flag"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/lib/pq"
)

type connectionRetryingDriver struct {
	driver.Driver
	driverName   string
	readTimeout  time.Duration
	writeTimeout time.Duration
}

func SetupConnectionRetryingDriver(
	delegateDriverName string,
	dbConfig flag.DBConfig,
	newDriverName string,
) {
	for _, driverName := range sql.Drivers() {
		if driverName == newDriverName {
			return
		}
	}
	delegateDBConn, err := sql.Open(delegateDriverName, dbConfig.ConnectionString())
	if err == nil {
		// ignoring any connection errors since we only need this to access the driver struct
		_ = delegateDBConn.Close()
	}

	connectionRetryingDriver := &connectionRetryingDriver{
		delegateDBConn.Driver(),
		delegateDriverName,
		dbConfig.GetReadTimeout(),
		dbConfig.GetWriteTimeout(),
	}
	sql.Register(newDriverName, connectionRetryingDriver)
}

func (d *connectionRetryingDriver) Open(name string) (driver.Conn, error) {
	var conn driver.Conn

	err := backoff.Retry(func() error {
		var err error
		if d.driverName == "postgres" {
			conn, err = pq.DialOpen(&timeoutDialer{
				readTimeout:  d.readTimeout,
				writeTimeout: d.writeTimeout,
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
