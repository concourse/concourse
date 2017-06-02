package migrations

import (
	"database/sql"
	"reflect"
	"runtime"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc/db/migration"
)

//go:generate counterfeiter . LimitedTx

type LimitedTx interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

func WithLogger(logger lager.Logger, mig migration.Migrator) migration.Migrator {
	fullName := runtime.FuncForPC(reflect.ValueOf(mig).Pointer()).Name()
	i := strings.LastIndex(fullName, ".")

	name := "unknown migration"
	if i >= 0 {
		name = fullName[i+1:]
	}

	logger = logger.Session("migrating", lager.Data{
		"migration": name,
	})

	return func(tx migration.LimitedTx) error {
		logger.Info("starting-migration")

		start := time.Now()
		defer func() {
			logger.Info("finishing-migration", lager.Data{"duration": time.Since(start).String()})
		}()

		return mig(tx)
	}
}

func Translogrifier(logger lager.Logger, migrations []migration.Migrator) []migration.Migrator {
	loggingMigrations := make([]migration.Migrator, len(migrations))

	for i, migration := range migrations {
		loggingMigrations[i] = WithLogger(logger, migration)
	}

	return loggingMigrations
}
