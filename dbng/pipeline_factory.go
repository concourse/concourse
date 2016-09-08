package dbng

type PipelineFactory struct {
	conn Conn
}

func NewPipelineFactory(conn Conn) *PipelineFactory {
	return &PipelineFactory{
		conn: conn,
	}
}

func (factory *PipelineFactory) CreatePipeline(team *Team, name string, config string) (*Pipeline, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var pipelineID int
	err = psql.Insert("pipelines").
		Columns("team_id", "name", "config").
		Values(team.ID, name, config).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&pipelineID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Pipeline{
		ID: pipelineID,
	}, nil
}
