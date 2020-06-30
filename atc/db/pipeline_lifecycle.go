package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

type PipelineLifecycle interface {
	ArchiveAbandonedPipelines() (int, error)
}

func NewPipelineLifecycle(conn Conn, lockFactory lock.LockFactory) PipelineLifecycle {
	return &pipelineLifecycle{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

type pipelineLifecycle struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func (pl *pipelineLifecycle) ArchiveAbandonedPipelines() (int, error) {
	tx, err := pl.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)

	latestSuccessfulBuilds := `WITH successful_builds AS (
      SELECT DISTINCT ON (job_id) id, job_id
      FROM builds
      WHERE status='succeeded'
      ORDER BY job_id, id DESC
      )`
	rows, err := pipelinesQuery.
		Prefix(latestSuccessfulBuilds).
		LeftJoin("jobs j ON j.id = p.parent_job_id").
		LeftJoin("successful_builds ON successful_builds.job_id = p.parent_job_id").
		Where(sq.Or{
			// pipeline is no longer set by the parent pipeline
			sq.And{
				sq.NotEq{"parent_build_id": nil},
				sq.Eq{"archived": false},
				sq.Expr("p.parent_build_id < successful_builds.id"),
			},
			// parent pipeline was destroyed
			sq.And{
				sq.NotEq{"parent_build_id": nil},
				sq.Eq{"j.id": nil},
			},
			// pipeline was set by a job. The job was removed from the parent pipeline
			sq.Eq{"j.active": false},
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var archivedPipelines []pipeline
	for rows.Next() {
		p := newPipeline(pl.conn, pl.lockFactory)
		if err = scanPipeline(p, rows); err != nil {
			return 0, err
		}

		archivedPipelines = append(archivedPipelines, *p)
	}

	for _, pipeline := range archivedPipelines {
		err = pipeline.archive(tx)
		if err != nil {
			return 0, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(archivedPipelines), nil
}
