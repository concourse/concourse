package batch

import (
	"context"
	"database/sql"
	"errors"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	_ "github.com/lib/pq"

	"github.com/concourse/concourse/atc/db"
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

type BuildEventsBigintMigrator struct {
	DB        db.Conn
	BatchSize int
}

func (migrator BuildEventsBigintMigrator) Migrate(ctx context.Context) (bool, error) {
	logger := lagerctx.FromContext(ctx).Session("migrate")

	logger.Debug("start")
	defer logger.Debug("done")

	var highBuild, highEvent int
	err := psql.Select("build_id_old", "event_id").
		From("build_events").
		Where(sq.Eq{"build_id": nil}).
		OrderBy("build_id_old DESC", "event_id DESC").
		Limit(1).
		RunWith(migrator.DB).
		QueryRowContext(ctx).
		Scan(&highBuild, &highEvent)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Debug("nothing-to-migrate")
			return true, nil
		}

		var pqErr *pq.Error
		// NB: unfortunately this approach will result in a postgresql server-side
		// error log, but there's no guarantee that we have permissions to inspect
		// the schema, so i don't know of another option :/
		if errors.As(err, &pqErr) && pqErr.Code.Name() == "undefined_column" {
			logger.Debug("already-migrated")
			return false, nil
		}

		return false, err
	}

	var lowBuild, lowEvent int
	err = psql.Select("build_id_old", "event_id").
		From("build_events").
		Where(sq.Eq{"build_id": nil}).
		Where(sq.Expr("(build_id_old, event_id) <= (?, ?)", highBuild, highEvent)).
		OrderBy("build_id_old DESC", "event_id DESC").
		Offset(uint64(migrator.BatchSize)).
		Limit(1).
		RunWith(migrator.DB).
		QueryRowContext(ctx).
		Scan(&lowBuild, &lowEvent)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Info("last-batch")
		} else {
			return false, err
		}
	}

	logger.Info("migrating", lager.Data{
		"from": []int{highBuild, highEvent},
		"to":   []int{lowBuild, lowEvent},
	})

	result, err := psql.Update("build_events").
		Set("build_id", sq.Expr("build_id_old")).
		Where(sq.Eq{"build_id": nil}).
		Where(sq.Expr("(build_id_old, event_id) <= (?, ?)", highBuild, highEvent)).
		Where(sq.Expr("(build_id_old, event_id) > (?, ?)", lowBuild, lowEvent)).
		RunWith(migrator.DB).
		ExecContext(ctx)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	logger.Debug("rows-affected", lager.Data{
		"rows": rows,
	})

	return false, nil
}

func (migrator BuildEventsBigintMigrator) Cleanup(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("cleanup")

	logger.Debug("start")
	defer logger.Debug("done")

	tx, err := migrator.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
    ALTER TABLE build_events
		ADD CONSTRAINT build_events_build_id_event_id_pkey PRIMARY KEY USING INDEX build_events_build_id_event_id
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `ALTER TABLE build_events DROP COLUMN build_id_old`)
	if err != nil {
		return err
	}

	return tx.Commit()
}
