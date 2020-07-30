package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineLifecycle

type PipelineLifecycle interface {
	ArchiveAbandonedPipelines() error
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

func (pl *pipelineLifecycle) ArchiveAbandonedPipelines() error {
	tx, err := pl.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	rows, err := pipelinesQuery.
		LeftJoin("jobs j ON j.id = p.parent_job_id").
		LeftJoin("pipelines p2 ON j.pipeline_id = p2.id").
		Where(sq.Or{
			// parent pipeline was destroyed
			sq.And{
				sq.NotEq{"p.parent_build_id": nil},
				sq.Eq{"j.id": nil},
			},
			// pipeline was set by a job. The job was removed from the parent pipeline
			sq.Eq{"j.active": false},
			// parent pipeline was archived
			sq.Eq{"p2.archived": true},
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}
	defer rows.Close()

	err = archivePipelines(tx, pl.conn, pl.lockFactory, rows)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func archivePipelines(tx Tx, conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) error {
	var toArchive []pipeline
	for rows.Next() {
		p := newPipeline(conn, lockFactory)
		if err := scanPipeline(p, rows); err != nil {
			return err
		}

		toArchive = append(toArchive, *p)
	}

	for _, pipeline := range toArchive {
		err := pipeline.archive(tx)
		if err != nil {
			return err
		}
	}

	return nil
}
