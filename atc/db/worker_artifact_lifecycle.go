package db

import (
	sq "github.com/Masterminds/squirrel"
)

//counterfeiter:generate . WorkerArtifactLifecycle
type WorkerArtifactLifecycle interface {
	RemoveExpiredArtifacts() error
}

type artifactLifecycle struct {
	conn DbConn
}

func NewArtifactLifecycle(conn DbConn) *artifactLifecycle {
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
