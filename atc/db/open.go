package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/lib/pq"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Bus() NotificationsBus
	EncryptionStrategy() encryption.Strategy

	Ping() error
	Driver() driver.Driver

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

//go:generate counterfeiter . Tx

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

func Open(logger lager.Logger, sqlDriver string, sqlDataSource string, newKey *encryption.Key, oldKey *encryption.Key, connectionName string, lockFactory lock.LockFactory) (Conn, error) {
	for {
		var strategy encryption.Strategy
		if newKey != nil {
			strategy = newKey
		} else {
			strategy = encryption.NewNoEncryption()
		}

		sqlDb, err := migration.NewOpenHelper(sqlDriver, sqlDataSource, lockFactory, strategy).Open()
		if err != nil {
			if shouldRetry(err) {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, err
		}

		switch {
		case oldKey != nil && newKey == nil:
			err = decryptToPlaintext(logger.Session("decrypt"), sqlDb, oldKey)
		case oldKey != nil && newKey != nil:
			err = encryptWithNewKey(logger.Session("rotate"), sqlDb, newKey, oldKey)
		}
		if err != nil {
			return nil, err
		}

		if newKey != nil {
			err = encryptPlaintext(logger.Session("encrypt"), sqlDb, newKey)
			if err != nil {
				return nil, err
			}
		}

		listener := pq.NewDialListener(keepAliveDialer{}, sqlDataSource, time.Second, time.Minute, nil)

		return &db{
			DB: sqlDb,

			bus:        NewNotificationsBus(listener, sqlDb),
			encryption: strategy,
			name:       connectionName,
		}, nil
	}
}

func shouldRetry(err error) bool {
	if strings.Contains(err.Error(), "dial ") {
		return true
	}

	if pqErr, ok := err.(*pq.Error); ok {
		return pqErr.Code.Name() == "cannot_connect_now"
	}

	return false
}

type encryptedColumn struct {
	Table      string
	Column     string
	PrimaryKey string
}

var encryptedColumns = []encryptedColumn{
	{"teams", "legacy_auth", "id"},
	{"resources", "config", "id"},
	{"jobs", "config", "id"},
	{"resource_types", "config", "id"},
	{"builds", "private_plan", "id"},
	{"cert_cache", "cert", "domain"},
	{"checks", "plan", "id"},
	{"pipelines", "var_sources", "id"},
}

func encryptPlaintext(logger lager.Logger, sqlDB *sql.DB, key *encryption.Key) error {
	for _, ec := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT ` + ec.PrimaryKey + `, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NULL
			AND ` + ec.Column + ` IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val        sql.NullString
			)

			err := rows.Scan(&primaryKey, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			if !val.Valid {
				continue
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			encrypted, nonce, err := key.Encrypt([]byte(val.String))
			if err != nil {
				rLog.Error("failed-to-encrypt", err)
				return err
			}

			_, err = sqlDB.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = $2
				WHERE `+ec.PrimaryKey+` = $3
			`, encrypted, nonce, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			encryptedRows++
		}

		if encryptedRows > 0 {
			tLog.Info("encrypted-existing-plaintext-data", lager.Data{
				"rows": encryptedRows,
			})
		}
	}

	return nil
}

func decryptToPlaintext(logger lager.Logger, sqlDB *sql.DB, oldKey *encryption.Key) error {
	for _, ec := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT ` + ec.PrimaryKey + `, nonce, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		decryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val, nonce string
			)

			err := rows.Scan(&primaryKey, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			decrypted, err := oldKey.Decrypt(val, &nonce)
			if err != nil {
				rLog.Error("failed-to-decrypt", err)
				return err
			}

			_, err = sqlDB.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = NULL
				WHERE `+ec.PrimaryKey+` = $2
			`, decrypted, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			decryptedRows++
		}

		if decryptedRows > 0 {
			tLog.Info("decrypted-existing-encrypted-data", lager.Data{
				"rows": decryptedRows,
			})
		}
	}

	return nil
}

var ErrEncryptedWithUnknownKey = errors.New("row encrypted with neither old nor new key")

func encryptWithNewKey(logger lager.Logger, sqlDB *sql.DB, newKey *encryption.Key, oldKey *encryption.Key) error {
	for _, ec := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT ` + ec.PrimaryKey + `, nonce, ` + ec.Column + `
			FROM ` + ec.Table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": ec.Table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				primaryKey interface{}
				val, nonce string
			)

			err := rows.Scan(&primaryKey, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"primary-key": primaryKey,
			})

			decrypted, err := oldKey.Decrypt(val, &nonce)
			if err != nil {
				_, err = newKey.Decrypt(val, &nonce)
				if err == nil {
					rLog.Debug("already-encrypted-with-new-key")
					continue
				}

				logger.Error("failed-to-decrypt-with-either-key", err)
				return ErrEncryptedWithUnknownKey
			}

			encrypted, newNonce, err := newKey.Encrypt(decrypted)
			if err != nil {
				rLog.Error("failed-to-encrypt", err)
				return err
			}

			_, err = sqlDB.Exec(`
				UPDATE `+ec.Table+`
				SET `+ec.Column+` = $1, nonce = $2
				WHERE `+ec.PrimaryKey+` = $3
			`, encrypted, newNonce, primaryKey)
			if err != nil {
				rLog.Error("failed-to-update", err)
				return err
			}

			encryptedRows++
		}

		if encryptedRows > 0 {
			tLog.Info("re-encrypted-existing-encrypted-data", lager.Data{
				"rows": encryptedRows,
			})
		}
	}

	return nil
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

	session            *ConnectionSession
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
