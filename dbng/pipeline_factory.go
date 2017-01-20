package dbng

import "github.com/concourse/atc/db/lock"

//go:generate counterfeiter . PipelineFactory

type PipelineFactory interface {
	GetPipelineByID(teamID int, pipelineID int) Pipeline
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

func (f *pipelineFactory) GetPipelineByID(teamID int, pipelineID int) Pipeline {
	return &pipeline{
		id:          pipelineID,
		teamID:      teamID,
		conn:        f.conn,
		lockFactory: f.lockFactory,
	}
}
