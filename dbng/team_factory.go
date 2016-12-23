package dbng

import (
	"database/sql"

	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . TeamFactory

type TeamFactory interface {
	CreateTeam(name string) (Team, error)
	FindTeam(name string) (Team, bool, error)
}

type teamFactory struct {
	conn Conn
}

func NewTeamFactory(conn Conn) TeamFactory {
	return &teamFactory{
		conn: conn,
	}
}

func (factory *teamFactory) CreateTeam(name string) (Team, error) {
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

	return &team{
		ID:   teamID,
		conn: factory.conn,
	}, nil
}

func (factory *teamFactory) FindTeam(name string) (Team, bool, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var teamID int
	err = psql.Select("id").
		From("teams").
		Where(sq.Eq{"name": name}).
		RunWith(tx).
		QueryRow().
		Scan(&teamID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return &team{
		ID:   teamID,
		conn: factory.conn,
	}, true, nil
}
