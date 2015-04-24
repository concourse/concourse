package db

import (
	"database/sql"

	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . PipelineDBFactory

type PipelineDBFactory interface {
	Build(pipeline SavedPipeline) PipelineDB
	BuildWithName(pipelineName string) (PipelineDB, error)
}

type pipelineDBFactory struct {
	logger lager.Logger

	conn        *sql.DB
	bus         *notificationsBus
	pipelinesDB PipelinesDB
}

func NewPipelineDBFactory(
	logger lager.Logger,
	sqldbConnection *sql.DB,
	bus *notificationsBus,
	pipelinesDB PipelinesDB,
) *pipelineDBFactory {
	return &pipelineDBFactory{
		logger: logger,

		conn:        sqldbConnection,
		bus:         bus,
		pipelinesDB: pipelinesDB,
	}
}

func (pdbf *pipelineDBFactory) BuildWithName(pipelineName string) (PipelineDB, error) {
	savedPipeline, err := pdbf.pipelinesDB.GetPipelineByName(pipelineName)
	if err != nil {
		return nil, err
	}

	return pdbf.Build(savedPipeline), nil
}

func (pdbf *pipelineDBFactory) Build(pipeline SavedPipeline) PipelineDB {
	return &pipelineDB{
		logger: pdbf.logger,

		conn: pdbf.conn,
		bus:  pdbf.bus,

		SavedPipeline: pipeline,
	}
}
