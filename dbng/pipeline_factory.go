package dbng

import (
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineFactory

type PipelineFactory interface {
	GetPipelineByID(teamID int, pipelineID int) Pipeline
	PublicPipelines() ([]Pipeline, error)
	AllPipelines() ([]Pipeline, error)
}

type pipelineFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
	encryption  EncryptionStrategy
}

func NewPipelineFactory(conn Conn, lockFactory lock.LockFactory, encryption EncryptionStrategy) PipelineFactory {
	return &pipelineFactory{
		conn:        conn,
		lockFactory: lockFactory,
		encryption:  encryption,
	}
}

func (f *pipelineFactory) GetPipelineByID(teamID int, pipelineID int) Pipeline {
	// XXX: construct a real one using the regular pipeline constructors; don't just set teamID etc inline
	return &pipeline{
		id:          pipelineID,
		teamID:      teamID,
		conn:        f.conn,
		lockFactory: f.lockFactory,
		encryption:  f.encryption,
	}
}

func (f *pipelineFactory) PublicPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		Where(sq.Eq{"p.public": true}).
		OrderBy("t.name, ordering").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	pipelines, err := scanPipelines(f.conn, f.lockFactory, f.encryption, rows)
	if err != nil {
		return nil, err
	}

	return pipelines, nil
}

func (f *pipelineFactory) AllPipelines() ([]Pipeline, error) {
	rows, err := pipelinesQuery.
		OrderBy("ordering").
		RunWith(f.conn).
		Query()
	if err != nil {
		return nil, err
	}

	return scanPipelines(f.conn, f.lockFactory, f.encryption, rows)
}
