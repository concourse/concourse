package db

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineFactory

type PipelineFactory interface {
	VisiblePipelines([]string) ([]Pipeline, error)
	AllPipelines() ([]Pipeline, error)
	PipelinesToSchedule() ([]Pipeline, error)
	GetPipeline(int) (Pipeline, bool, error)
}

type pipelineFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewPipelineFactory(conn Conn, lockFactory lock.LockFactory) PipelineFactory {
	return &pipelineFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (f *pipelineFactory) VisiblePipelines(teamNames []string) ([]Pipeline, error) {
	tx, err := f.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	rows, err := pipelinesQuery.
		Where(sq.Eq{"t.name": teamNames}).
		OrderBy("t.name ASC", "ordering ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	currentTeamPipelines, err := scanPipelines(f.conn, f.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	rows, err = pipelinesQuery.
		Where(sq.NotEq{"t.name": teamNames}).
		Where(sq.Eq{"public": true}).
		OrderBy("t.name ASC", "ordering ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	otherTeamPublicPipelines, err := scanPipelines(f.conn, f.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return append(currentTeamPipelines, otherTeamPublicPipelines...), nil
}

func (f *pipelineFactory) AllPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		OrderBy("t.name ASC", "ordering ASC").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, rows)
}

func (f *pipelineFactory) PipelinesToSchedule() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Join("jobs j ON j.pipeline_id = p.id").
		Where(sq.Expr("j.schedule_requested > j.last_scheduled")).
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, rows)
}

func (f *pipelineFactory) GetPipeline(id int) (Pipeline, bool, error) {
	row := pipelinesQuery.
		Where(sq.Eq{"p.id": id}).
		RunWith(f.conn).
		QueryRow()
	p := newPipeline(f.conn, f.lockFactory)
	if err := scanPipeline(p, row); err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}
	return p, true, nil
}
