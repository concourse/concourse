package db

import "github.com/concourse/atc/db/lock"

//go:generate counterfeiter . PipelineDBFactory

type PipelineDBFactory interface {
	Build(pipeline SavedPipeline) PipelineDB
}

type pipelineDBFactory struct {
	conn Conn
	bus  *notificationsBus

	lockFactory lock.LockFactory
}

func NewPipelineDBFactory(
	sqldbConnection Conn,
	bus *notificationsBus,
	lockFactory lock.LockFactory,
) *pipelineDBFactory {
	return &pipelineDBFactory{
		conn:        sqldbConnection,
		bus:         bus,
		lockFactory: lockFactory,
	}
}

func (pdbf *pipelineDBFactory) Build(pipeline SavedPipeline) PipelineDB {
	return &pipelineDB{
		conn: pdbf.conn,
		bus:  pdbf.bus,

		buildFactory: newBuildFactory(pdbf.conn, pdbf.bus, pdbf.lockFactory),
		lockFactory:  pdbf.lockFactory,

		SavedPipeline: pipeline,
	}
}
