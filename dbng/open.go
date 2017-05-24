package dbng

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/migrations"
	"github.com/concourse/atc/dbng/migration"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/lib/pq"
)

type Conn interface {
	Bus() NotificationsBus
	EncryptionStrategy() EncryptionStrategy

	Begin() (Tx, error)
	Driver() driver.Driver
	Exec(query string, args ...interface{}) (sql.Result, error)
	Ping() error
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats

	Close() error
}

type Tx interface {
	Commit() error
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) squirrel.RowScanner
	Rollback() error
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func Open(logger lager.Logger, sqlDriver string, sqlDataSource string, encryption EncryptionStrategy) (Conn, error) {
	for {
		sqlDb, err := migration.Open(sqlDriver, sqlDataSource, migrations.Migrations)
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db-retrying", err)
				time.Sleep(5 * time.Second)
				continue
			}

			return nil, err
		}

		for table, col := range map[string]string{
			"teams":          "auth",
			"resources":      "config",
			"jobs":           "config",
			"resource_types": "config",
			"pipelines":      "config",
		} {
			rows, err := sqlDb.Query(`
			SELECT id, ` + col + `
			FROM ` + table + `
			WHERE nonce IS NULL
		`)
			if err != nil && err != sql.ErrNoRows {
				return nil, err
			}

			for rows.Next() {
				var (
					id  int
					val sql.NullString
				)

				err := rows.Scan(&id, &val)
				if err != nil {
					return nil, err
				}

				if !val.Valid {
					continue
				}

				encrypted, nonce, err := encryption.Encrypt([]byte(val.String))
				if err != nil {
					return nil, err
				}

				_, err = sqlDb.Exec(`
				UPDATE `+table+`
				SET `+col+` = $1, nonce = $2
				WHERE id = $3
			`, encrypted, nonce, id)
				if err != nil {
					return nil, err
				}
			}
		}

		listener := pq.NewListener(sqlDataSource, time.Second, time.Minute, nil)

		return &db{
			DB: sqlDb,

			bus:        NewNotificationsBus(listener, sqlDb),
			encryption: encryption,
		}, nil
	}
}

type db struct {
	*sql.DB

	bus        NotificationsBus
	encryption EncryptionStrategy
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

func (db *db) Begin() (Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}

	return &dbTx{tx}, nil
}

// to conform to squirrel.Runner interface
func (db *db) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return db.DB.QueryRow(query, args...)
}

type dbTx struct {
	*sql.Tx
}

// to conform to squirrel.Runner interface
func (tx *dbTx) QueryRow(query string, args ...interface{}) squirrel.RowScanner {
	return tx.Tx.QueryRow(query, args...)
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}
