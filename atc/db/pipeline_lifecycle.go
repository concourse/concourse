package db

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . PipelineLifecycle
type PipelineLifecycle interface {
	ArchiveAbandonedPipelines() error
	RemoveBuildEventsForDeletedPipelines() error
}

func NewPipelineLifecycle(conn DbConn, lockFactory lock.LockFactory) PipelineLifecycle {
	return &pipelineLifecycle{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

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

	err = archivePipelines(tx, pl.conn, pl.lockFactory, pipelinesToArchive)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func archivePipelines(tx Tx, conn DbConn, lockFactory lock.LockFactory, rows *sql.Rows) error {
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
