package dbng

import (
	sq "github.com/Masterminds/squirrel"
)

type BuildFactory struct {
	conn Conn
}

func NewBuildFactory(conn Conn) *BuildFactory {
	return &BuildFactory{
		conn: conn,
	}
}

func (factory *BuildFactory) CreateOneOffBuild(team *Team) (*Build, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	var buildID int
	err = psql.Insert("builds").
		Columns("team_id", "name", "status").
		Values(team.ID, sq.Expr("nextval('one_off_name')"), "pending").
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		// TODO: explicitly handle fkey constraint
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Build{
		ID: buildID,
	}, nil
}
