package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineFactory

type PipelineFactory interface {
	VisiblePipelines([]string) ([]Pipeline, error)
	AllPipelines() ([]Pipeline, error)
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
	rows, err := pipelinesQuery.
		Where(sq.Or{
			sq.Eq{"t.name": teamNames},
			sq.Eq{"public": true},
		}).
		OrderBy("public ASC", "team_id ASC", "ordering ASC").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, rows)
}

func (f *pipelineFactory) AllPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		OrderBy("team_id ASC", "ordering ASC").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, rows)
}
