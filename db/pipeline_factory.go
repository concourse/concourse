package db

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/lock"
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
		Where(sq.Eq{"t.name": teamNames}).
		OrderBy("team_id ASC", "ordering ASC").
		RunWith(f.conn).
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
		OrderBy("team_id ASC", "ordering ASC").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	otherTeamPublicPipelines, err := scanPipelines(f.conn, f.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	return append(currentTeamPipelines, otherTeamPublicPipelines...), nil
}

func (f *pipelineFactory) AllPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		OrderBy("ordering").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, rows)
}
