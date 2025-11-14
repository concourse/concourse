package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"slices"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type connectionRetryingDriver struct {
	driver.Driver
	driverName string
}

func SetupConnectionRetryingDriver(
	delegateDriverName string,
	sqlDataSource string,
	newDriverName string,
) {
	if slices.Contains(sql.Drivers(), newDriverName) {
		return
	}
	delegateDBConn, err := sql.Open(delegateDriverName, sqlDataSource)
	if err == nil {
		// ignoring any connection errors since we only need this to access the driver struct
		_ = delegateDBConn.Close()
	}

	connectionRetryingDriver := &connectionRetryingDriver{
		delegateDBConn.Driver(),
		delegateDriverName,
	}
	sql.Register(newDriverName, connectionRetryingDriver)
}

func (d *connectionRetryingDriver) Open(connStr string) (driver.Conn, error) {
	conn, err := backoff.Retry(context.TODO(), func() (driver.Conn, error) {
		conn, err := d.Driver.Open(connStr)
		if err != nil {
			var pgErr *pgconn.PgError
			var connErr *pgconn.ConnectError
			if errors.As(err, &pgErr) || errors.As(err, &connErr) || pgconn.SafeToRetry(err) {
				return nil, err
			}

			return nil, backoff.Permanent(err)
		}

		return conn, nil
	},
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(30*time.Second),
	)

	if err != nil {
		return nil, err
	}

	return conn, nil
}
