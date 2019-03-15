package db

import (
	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . WorkerArtifactLifecycle

type WorkerArtifactLifecycle interface {
	RemoveExpiredArtifacts() error
}

type artifactLifecycle struct {
	conn Conn
}

func NewArtifactLifecycle(conn Conn) *artifactLifecycle {
	return &artifactLifecycle{
		conn: conn,
	}
}

func (lifecycle *artifactLifecycle) RemoveExpiredArtifacts() error {

	_, err := psql.Delete("worker_artifacts").
		Where(sq.Expr("created_at < NOW() - interval '12 hours'")).
		RunWith(lifecycle.conn).
		Exec()

	return err
}
