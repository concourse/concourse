package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineLifecycle

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

	rows, err := pipelinesQuery.
		LeftJoin("jobs j ON j.id = p.parent_job_id").
		Where(sq.Or{
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

	archivedPipelines, err := archivePipelines(tx, pl.conn, pl.lockFactory, rows)
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}

	return len(archivedPipelines), nil
}

func archivePipelines(tx Tx, conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) ([]pipeline, error) {
	var archivedPipelines []pipeline
	for rows.Next() {
		p := newPipeline(conn, lockFactory)
		if err := scanPipeline(p, rows); err != nil {
			return nil, err
		}

		archivedPipelines = append(archivedPipelines, *p)
	}

	for _, pipeline := range archivedPipelines {
		err := pipeline.archive(tx)
		if err != nil {
			return nil, err
		}
	}

	return archivedPipelines, nil
}
