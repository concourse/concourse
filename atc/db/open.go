package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/lager/v3"
	"github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//counterfeiter:generate . DbConn
type DbConn interface {
	Bus() NotificationsBus
	EncryptionStrategy() encryption.Strategy

	Ping() error
	Driver() driver.Driver
	Conn(context.Context) (*sql.Conn, error)

	Begin() (Tx, error)
	Exec(string, ...interface{}) (sql.Result, error)
	Prepare(string) (*sql.Stmt, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) squirrel.RowScanner

	BeginTx(context.Context, *sql.TxOptions) (Tx, error)
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) squirrel.RowScanner

	SetMaxIdleConns(int)
	SetMaxOpenConns(int)
	Stats() sql.DBStats

	Close() error
	Name() string
}

//counterfeiter:generate . Tx
type Tx interface {
	Commit() error
	Exec(string, ...interface{}) (sql.Result, error)
	Prepare(string) (*sql.Stmt, error)
	Query(string, ...interface{}) (*sql.Rows, error)
	QueryRow(string, ...interface{}) squirrel.RowScanner
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) squirrel.RowScanner
	Rollback() error
	Stmt(*sql.Stmt) *sql.Stmt
	EncryptionStrategy() encryption.Strategy
}

func Open(logger lager.Logger, driver, dsn string, newKey, oldKey *encryption.Key, name string, lockFactory lock.LockFactory) (DbConn, error) {
	for {
		sqlDB, err := migration.NewOpenHelper(driver, dsn, lockFactory, newKey, oldKey).Open()
		if err != nil {
			if shouldRetry(err) {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, err
		}

		return NewConn(name, sqlDB, dsn, oldKey, newKey)
	}
}

func NewConn(name string, sqlDB *sql.DB, dsn string, oldKey, newKey *encryption.Key) (DbConn, error) {
	// only used for the LISTEN/NOTIFY commands
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}

	listener := NewPgxListener(pool)

	var strategy encryption.Strategy
	if newKey != nil {
		strategy = newKey
	} else {
		strategy = encryption.NewNoEncryption()
	}

	return &db{
		DB: sqlDB,

		bus:        NewNotificationsBus(listener, sqlDB),
		encryption: strategy,
		name:       name,
	}, nil
}

func shouldRetry(err error) bool {
	return pgconn.SafeToRetry(err)
}

type db struct {
	*sql.DB

	bus        NotificationsBus
	encryption encryption.Strategy
	name       string
}

func (db *db) Name() string {
	return db.name
}

func (db *db) Bus() NotificationsBus {
	return db.bus
}

func (db *db) EncryptionStrategy() encryption.Strategy {
	return db.encryption
}

func (db *db) Close() error {
	var errs error
	dbErr := db.DB.Close()
	if dbErr != nil {
		errs = multierror.Append(errs, dbErr)
	}

	busErr := db.bus.Close()
	if busErr != nil {
		errs = multierror.Append(errs, busErr)
	}

	return errs
}

// Close ignores errors, and should used with defer.
// makes errcheck happy that those errs are captured
func Close(c io.Closer) {
	_ = c.Close()
}

func (db *db) Begin() (Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &dbTx{tx, GlobalConnectionTracker.Track(), db.EncryptionStrategy()}, nil
}

func (db *db) Exec(query string, args ...interface{}) (sql.Result, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.Exec(query, args...)
}

func (db *db) Prepare(query string) (*sql.Stmt, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.Prepare(query)
}

func (db *db) Query(query string, args ...interface{}) (*sql.Rows, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.Query(query, args...)
}

// to conform to squirrel.Runner interface
func (db *db) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.QueryRow(query, args...)
}

func (db *db) BeginTx(ctx context.Context, opts *sql.TxOptions) (Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}

	return &dbTx{tx, GlobalConnectionTracker.Track(), db.EncryptionStrategy()}, nil
}

func (db *db) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.ExecContext(ctx, query, args...)
}

func (db *db) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.PrepareContext(ctx, query)
}

func (db *db) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.QueryContext(ctx, query, args...)
}

// to conform to squirrel.Runner interface
func (db *db) QueryRowContext(ctx context.Context, query string, args ...interface{}) squirrel.RowScanner {
	defer GlobalConnectionTracker.Track().Release()
	return db.DB.QueryRowContext(ctx, query, args...)
}

type dbTx struct {
	*sql.Tx

	session            ConnectionSession
	encryptionStrategy encryption.Strategy
}

// to conform to squirrel.Runner interface
func (tx *dbTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return tx.Tx.QueryRow(query, args...)
}

func (tx *dbTx) QueryRowContext(ctx context.Context, query string, args ...interface{}) squirrel.RowScanner {
	return tx.Tx.QueryRowContext(ctx, query, args...)
}

func (tx *dbTx) Commit() error {
	defer tx.session.Release()
	return tx.Tx.Commit()
}

func (tx *dbTx) Rollback() error {
	defer tx.session.Release()
	return tx.Tx.Rollback()
}

func (tx *dbTx) EncryptionStrategy() encryption.Strategy {
	return tx.encryptionStrategy
}

// Rollback ignores errors, and should be used with defer.
// makes errcheck happy that those errs are captured
func Rollback(tx Tx) {
	_ = tx.Rollback()
}

type NonOneRowAffectedError struct {
	RowsAffected int64
}

func (err NonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}
