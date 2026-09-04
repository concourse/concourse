package db

import (
	"database/sql"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . PipelineLifecycle
type PipelineLifecycle interface {
	// Finds any pipelines no longer being set by their parent pipeline via the
	// set_pipeline step and archives them
	ArchiveAbandonedPipelines() error
	RemoveBuildEventsForDeletedPipelines() error
	// Destroys any pipelines that have been archived for longer than the given
	// duration. The duration must be > 0. A duration of zero is a no-op.
	DestroyArchivedPipelines(time.Duration) error
}

func NewPipelineLifecycle(conn DbConn, lockFactory lock.LockFactory) PipelineLifecycle {
	return &pipelineLifecycle{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

var _ PipelineLifecycle = (*pipelineLifecycle)(nil)

type pipelineLifecycle struct {
	conn        DbConn
	lockFactory lock.LockFactory
}

func (pl *pipelineLifecycle) ArchiveAbandonedPipelines() error {
	tx, err := pl.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	pipelinesToArchive, err := pipelinesQuery.
		LeftJoin("jobs j ON (j.id = p.parent_job_id)").
		LeftJoin("pipelines parent ON (j.pipeline_id = parent.id)").
		Where(sq.And{
			// pipeline was set by some build
			sq.NotEq{"p.parent_job_id": nil},
			// pipeline is not already archived
			sq.Eq{"p.archived": false},
			sq.Or{
				// job (that set child pipeline) from parent pipeline is
				// removed, Concourse marks job as inactive
				sq.Eq{"j.active": false},
				// parent pipeline was destroyed, entire job record is gone
				sq.Eq{"j.id": nil},
				// parent pipeline was archived
				sq.Eq{"parent.archived": true},
				// build that set the pipeline is not the most recent for the job.
				// parent_build_id can be later than latest_completed_build_id if this
				// gc query runs during a run of a build, specifically between the time
				// of a completed set pipeline step and the build finishing. Also only
				// take into account successful builds in order to know if this child
				// pipeline had its set_pipeline step removed from parent job.
				sq.And{
					sq.Expr("p.parent_build_id < j.latest_completed_build_id"),
					sq.Expr(`EXISTS (
						SELECT 1
						FROM builds lb
						WHERE lb.id = j.latest_completed_build_id
						AND lb.status = ?
					)`, BuildStatusSucceeded),
				},
			},
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}
	defer pipelinesToArchive.Close()

	_, err = archivePipelines(tx, pl.conn, pl.lockFactory, pipelinesToArchive)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func archivePipelines(tx Tx, conn DbConn, lockFactory lock.LockFactory, rows *sql.Rows) ([]pipeline, error) {
	var toArchive []pipeline
	for rows.Next() {
		p := newPipeline(conn, lockFactory)
		if err := scanPipeline(p, rows); err != nil {
			return nil, err
		}

		toArchive = append(toArchive, *p)
	}

	for _, pipeline := range toArchive {
		err := pipeline.archive(tx)
		if err != nil {
			return nil, err
		}
	}

	return toArchive, nil
}

func (p *pipelineLifecycle) RemoveBuildEventsForDeletedPipelines() error {
	rows, err := psql.Select("id").
		From("deleted_pipelines").
		RunWith(p.conn).
		Query()
	if err != nil {
		return err
	}

	var idsToDelete []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
		idsToDelete = append(idsToDelete, id)
	}

	rows.Close()

	if len(idsToDelete) == 0 {
		return nil
	}

	for _, id := range idsToDelete {
		_, err = p.conn.Exec(fmt.Sprintf("DROP TABLE IF EXISTS pipeline_build_events_%d", id))
		if err != nil {
			return err
		}
	}

	_, err = psql.Delete("deleted_pipelines").
		Where(sq.Eq{"id": idsToDelete}).
		RunWith(p.conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (pl *pipelineLifecycle) DestroyArchivedPipelines(olderThan time.Duration) error {
	if olderThan <= 0 {
		return nil
	}

	tx, err := pl.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	archivedPipelinesToDestroy, err := pipelinesQuery.
		Where(sq.And{
			sq.Eq{"p.archived": true},
			sq.Expr(fmt.Sprintf("paused_at < NOW() - '%d seconds'::INTERVAL", int(olderThan.Seconds()))),
		}).
		RunWith(tx).
		Query()

	if err != nil {
		return err
	}
	defer archivedPipelinesToDestroy.Close()

	err = destroyArchivedPipelines(tx, pl.conn, pl.lockFactory, archivedPipelinesToDestroy)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func destroyArchivedPipelines(tx Tx, conn DbConn, lockFactory lock.LockFactory, rows *sql.Rows) error {
	var toDestroy []pipeline
	for rows.Next() {
		p := newPipeline(conn, lockFactory)
		if err := scanPipeline(p, rows); err != nil {
			return err
		}

		toDestroy = append(toDestroy, *p)
	}

	for _, pipeline := range toDestroy {
		err := pipeline.destroy(tx)
		if err != nil {
			return err
		}
	}

	return nil
}
