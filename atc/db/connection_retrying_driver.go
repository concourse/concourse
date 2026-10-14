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
	"github.com/jackc/pgx/v5/stdlib"
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

	return &retryingConn{inner: conn}, nil
}

// failoverCodes are SQLSTATE codes that indicate the connection has become
// unusable due to a failover event. Returning driver.ErrBadConn causes
// database/sql to discard the connection and transparently retry on a new one.
//
// 25006: read_only_sql_transaction — primary became standby; no mutation applied.
// 57P01: admin_shutdown — server shutting down; connection is dead.
// 57P02: crash_shutdown — server crashed; connection is dead.
var failoverCodes = map[string]bool{
	"25006": true,
	"57P01": true,
	"57P02": true,
}

func IsFailoverError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && failoverCodes[pgErr.Code]
}

func toDriverErr(err error) error {
	if err != nil && IsFailoverError(err) {
		return driver.ErrBadConn
	}
	return err
}

// Verify interface compliance between pgx and our retryingConn. pgx exposes
// their driver.Conn as stdlib.Conn. Whatever interfaces stdlib.Conn implements
// should also be implemented by our retryingConn.
var _ driverConn = (*stdlib.Conn)(nil)
var _ driverConn = (*retryingConn)(nil)

//counterfeiter:generate . driverConn
type driverConn interface {
	driver.Conn
	driver.ConnBeginTx
	driver.ConnPrepareContext
	driver.ExecerContext
	driver.NamedValueChecker
	driver.Pinger
	driver.QueryerContext
	driver.SessionResetter
}

// wraps driver.Conn - failover errors converted to driver.ErrBadConn so that database/sql retries on a fresh one
type retryingConn struct {
	inner driver.Conn
}

func NewRetryingConn(inner driver.Conn) *retryingConn {
	return &retryingConn{inner: inner}
}

func (c *retryingConn) Prepare(query string) (driver.Stmt, error) {
	return c.inner.Prepare(query)
}

func (c *retryingConn) Close() error {
	return c.inner.Close()
}

func (c *retryingConn) Begin() (driver.Tx, error) {
	return c.inner.Begin()
}

func (c *retryingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ec, ok := c.inner.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := ec.ExecContext(ctx, query, args)
	return result, toDriverErr(err)
}

func (c *retryingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	qc, ok := c.inner.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := qc.QueryContext(ctx, query, args)
	return rows, toDriverErr(err)
}

// preserve pgx's type codec support (e.g. []string, []int, time.Time) through the wrapper;
// without it database/sql falls back to driver.DefaultParameterConverter which rejects any non-primitive slice type.
func (c *retryingConn) CheckNamedValue(nv *driver.NamedValue) error {
	checker, ok := c.inner.(driver.NamedValueChecker)
	if !ok {
		return driver.ErrSkip
	}
	return checker.CheckNamedValue(nv)
}

func (c *retryingConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	pc, ok := c.inner.(driver.ConnPrepareContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	return pc.PrepareContext(ctx, query)
}

func (c *retryingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	pc, ok := c.inner.(driver.ConnBeginTx)
	if !ok {
		return nil, driver.ErrSkip
	}
	return pc.BeginTx(ctx, opts)
}

func (c *retryingConn) Ping(ctx context.Context) error {
	p, ok := c.inner.(driver.Pinger)
	if !ok {
		return driver.ErrSkip
	}
	return p.Ping(ctx)
}

func (c *retryingConn) ResetSession(ctx context.Context) error {
	sr, ok := c.inner.(driver.SessionResetter)
	if !ok {
		return driver.ErrSkip
	}
	return sr.ResetSession(ctx)
}
