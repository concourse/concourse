package db

//go:generate counterfeiter . PipelineDBFactory

type PipelineDBFactory interface {
	Build(pipeline SavedPipeline) PipelineDB
}

type pipelineDBFactory struct {
	conn Conn
	bus  *notificationsBus
}

func NewPipelineDBFactory(
	sqldbConnection Conn,
	bus *notificationsBus,
) *pipelineDBFactory {
	return &pipelineDBFactory{
		conn: sqldbConnection,
		bus:  bus,
	}
}

func (pdbf *pipelineDBFactory) Build(pipeline SavedPipeline) PipelineDB {
	return &pipelineDB{
		conn: pdbf.conn,
		bus:  pdbf.bus,

		buildFactory: newBuildFactory(pdbf.conn, pdbf.bus),

		SavedPipeline: pipeline,
	}
}
