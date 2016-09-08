package dbng

type ResourceFactory struct {
	conn Conn
}

func NewResourceFactory(conn Conn) *ResourceFactory {
	return &ResourceFactory{
		conn: conn,
	}
}

func (factory *ResourceFactory) CreateResource(pipeline *Pipeline, name string, config string) (*Resource, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var resourceID int
	err = psql.Insert("resources").
		Columns("pipeline_id", "name", "config").
		Values(pipeline.ID, name, config).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&resourceID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Resource{
		ID: resourceID,
	}, nil
}
