package db

import sq "github.com/Masterminds/squirrel"

//go:generate counterfeiter . ResourceConfigCheckSessionLifecycle

type ResourceConfigCheckSessionLifecycle interface {
	CleanExpiredResourceConfigCheckSessions() error
	CleanUselessResourceConfigCheckSessions() error
}

type resourceConfigCheckSessionLifecycle struct {
	conn Conn
}

func NewResourceConfigCheckSessionLifecycle(conn Conn) ResourceConfigCheckSessionLifecycle {
	return resourceConfigCheckSessionLifecycle{
		conn: conn,
	}
}

func (lifecycle resourceConfigCheckSessionLifecycle) CleanUselessResourceConfigCheckSessions() error {
	usedByActiveUnpausedResources, _, err := sq.
		Select("rccs.id").
		Distinct().
		From("resource_config_check_sessions rccs").
		Join("resource_configs rc ON rccs.resource_config_id = rc.id").
		Join("resources r ON r.resource_config_id = rc.id").
		Where(sq.And{
			sq.Expr("r.paused = false"),
			sq.Expr("r.active = true"),
		}).
		ToSql()
	if err != nil {
		return err
	}

	_, err = sq.Delete("resource_config_check_sessions").
		Where("id NOT IN (" + usedByActiveUnpausedResources + ")").
		PlaceholderFormat(sq.Dollar).
		RunWith(lifecycle.conn).
		Exec()
	if err != nil {
		return err
	}

	return err
}

func (lifecycle resourceConfigCheckSessionLifecycle) CleanExpiredResourceConfigCheckSessions() error {
	_, err := psql.Delete("resource_config_check_sessions").
		Where(sq.Expr("expires_at < NOW()")).
		RunWith(lifecycle.conn).
		Exec()

	return err
}
