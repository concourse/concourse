package dbng

type TeamFactory struct {
	conn Conn
}

func NewTeamFactory(conn Conn) *TeamFactory {
	return &TeamFactory{
		conn: conn,
	}
}

func (factory *TeamFactory) CreateTeam(name string) (*Team, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var teamID int
	err = psql.Insert("teams").
		// TODO: should metadata just be JSON?
		Columns("name").
		Values(name).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&teamID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Team{
		ID: teamID,
	}, nil
}
