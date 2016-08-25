package db

//go:generate counterfeiter . PipelineDBFactory

type PipelineDBFactory interface {
	Build(pipeline SavedPipeline) PipelineDB
}

type pipelineDBFactory struct {
	conn Conn
	bus  *notificationsBus

	leaseFactory LeaseFactory
}

func NewPipelineDBFactory(
	sqldbConnection Conn,
	bus *notificationsBus,
	leaseFactory LeaseFactory,
) *pipelineDBFactory {
	return &pipelineDBFactory{
		conn:         sqldbConnection,
		bus:          bus,
		leaseFactory: leaseFactory,
	}
}

func (pdbf *pipelineDBFactory) Build(pipeline SavedPipeline) PipelineDB {
	return &pipelineDB{
		conn: pdbf.conn,
		bus:  pdbf.bus,

		buildFactory: newBuildFactory(pdbf.conn, pdbf.bus, pdbf.leaseFactory),
		leaseFactory: pdbf.leaseFactory,

		SavedPipeline: pipeline,
	}
}
