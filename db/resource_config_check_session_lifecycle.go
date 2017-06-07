package db

import sq "github.com/Masterminds/squirrel"

//go:generate counterfeiter . ResourceConfigCheckSessionLifecycle

type ResourceConfigCheckSessionLifecycle interface {
	CleanExpiredResourceConfigCheckSessions() error
}

type resourceConfigCheckSessionLifecycle struct {
	conn Conn
}

func NewResourceConfigCheckSessionLifecycle(conn Conn) ResourceConfigCheckSessionLifecycle {
	return resourceConfigCheckSessionLifecycle{
		conn: conn,
	}
}

func (lifecycle resourceConfigCheckSessionLifecycle) CleanExpiredResourceConfigCheckSessions() error {
	_, err := psql.Delete("resource_config_check_sessions").
		Where(sq.Expr("expires_at < NOW()")).
		RunWith(lifecycle.conn).
		Exec()

	return err
}
