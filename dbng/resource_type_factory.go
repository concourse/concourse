package dbng

type ResourceTypeFactory struct {
	conn Conn
}

func NewResourceTypeFactory(conn Conn) *ResourceTypeFactory {
	return &ResourceTypeFactory{
		conn: conn,
	}
}

func (factory *ResourceTypeFactory) CreateResourceType(pipeline *Pipeline, name string, typ string, config string) (*ResourceType, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var resourceTypeID int
	err = psql.Insert("resource_types").
		Columns("pipeline_id", "name", "type", "config").
		Values(pipeline.ID, name, typ, config).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&resourceTypeID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &ResourceType{
		ID: resourceTypeID,
	}, nil
}
