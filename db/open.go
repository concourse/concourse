package db

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/db/migration"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/lib/pq"
)

//go:generate counterfeiter . Conn

type Conn interface {
	Bus() NotificationsBus
	EncryptionStrategy() EncryptionStrategy

	Ping() error
	Driver() driver.Driver

	Begin() (Tx, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner

	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats

	Close() error
	Name() string
}

//go:generate counterfeiter . Tx

type Tx interface {
	Commit() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner
	Rollback() error
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func Open(logger lager.Logger, sqlDriver string, sqlDataSource string, newKey *EncryptionKey, oldKey *EncryptionKey, connectionName string, lockFactory lock.LockFactory) (Conn, error) {
	for {
		var strategy EncryptionStrategy
		if newKey != nil {
			strategy = newKey
		} else {
			strategy = NewNoEncryption()
		}

		sqlDb, err := migration.NewOpenHelper(sqlDriver, sqlDataSource, lockFactory).Open()
		if err != nil {
			if strings.Contains(err.Error(), "dial ") {
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

		listener := pq.NewListener(sqlDataSource, time.Second, time.Minute, nil)

		return &db{
			DB: sqlDb,

			bus:        NewNotificationsBus(listener, sqlDb),
			encryption: strategy,
			name:       connectionName,
		}, nil
	}
}

var encryptedColumns = map[string]string{
	"teams":          "auth",
	"resources":      "config",
	"jobs":           "config",
	"resource_types": "config",
	"builds":         "engine_metadata",
}

func encryptPlaintext(logger lager.Logger, sqlDB *sql.DB, key *EncryptionKey) error {
	for table, col := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT id, ` + col + `
			FROM ` + table + `
			WHERE nonce IS NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				id  int
				val sql.NullString
			)

			err := rows.Scan(&id, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			if !val.Valid {
				continue
			}

			rLog := tLog.Session("row", lager.Data{
				"id": id,
			})

			encrypted, nonce, err := key.Encrypt([]byte(val.String))
			if err != nil {
				rLog.Error("failed-to-encrypt", err)
				return err
			}

			_, err = sqlDB.Exec(`
				UPDATE `+table+`
				SET `+col+` = $1, nonce = $2
				WHERE id = $3
			`, encrypted, nonce, id)
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

func decryptToPlaintext(logger lager.Logger, sqlDB *sql.DB, oldKey *EncryptionKey) error {
	for table, col := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT id, nonce, ` + col + `
			FROM ` + table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": table,
		})

		decryptedRows := 0

		for rows.Next() {
			var (
				id         int
				val, nonce string
			)

			err := rows.Scan(&id, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"id": id,
			})

			decrypted, err := oldKey.Decrypt(val, &nonce)
			if err != nil {
				rLog.Error("failed-to-decrypt", err)
				return err
			}

			_, err = sqlDB.Exec(`
				UPDATE `+table+`
				SET `+col+` = $1, nonce = NULL
				WHERE id = $2
			`, decrypted, id)
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

func encryptWithNewKey(logger lager.Logger, sqlDB *sql.DB, newKey *EncryptionKey, oldKey *EncryptionKey) error {
	for table, col := range encryptedColumns {
		rows, err := sqlDB.Query(`
			SELECT id, nonce, ` + col + `
			FROM ` + table + `
			WHERE nonce IS NOT NULL
		`)
		if err != nil {
			return err
		}

		tLog := logger.Session("table", lager.Data{
			"table": table,
		})

		encryptedRows := 0

		for rows.Next() {
			var (
				id         int
				val, nonce string
			)

			err := rows.Scan(&id, &nonce, &val)
			if err != nil {
				tLog.Error("failed-to-scan", err)
				return err
			}

			rLog := tLog.Session("row", lager.Data{
				"id": id,
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
				UPDATE `+table+`
				SET `+col+` = $1, nonce = $2
				WHERE id = $3
			`, encrypted, newNonce, id)
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
	encryption EncryptionStrategy
	name       string
}

func (db *db) Name() string {
	return db.name
}

func (db *db) Bus() NotificationsBus {
	return db.bus
}

func (db *db) EncryptionStrategy() EncryptionStrategy {
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

	return &dbTx{tx, GlobalConnectionTracker.Track()}, nil
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

type dbTx struct {
	*sql.Tx

	session *ConnectionSession
}

// to conform to squirrel.Runner interface
func (tx *dbTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return tx.Tx.QueryRow(query, args...)
}

func (tx *dbTx) Commit() error {
	defer tx.session.Release()
	return tx.Tx.Commit()
}

func (tx *dbTx) Rollback() error {
	defer tx.session.Release()
	return tx.Tx.Rollback()
}

// Rollback ignores errors, and should be used with defer.
// makes errcheck happy that those errs are captured
func Rollback(tx Tx) {
	_ = tx.Rollback()
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}
