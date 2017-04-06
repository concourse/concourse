package dbng

import (
	"database/sql"
	"fmt"

	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . TeamFactory

type TeamFactory interface {
	CreateTeam(atc.Team) (Team, error)
	FindTeam(string) (Team, bool, error)
	GetByID(teamID int) Team
}

type teamFactory struct {
	conn        Conn
	lockFactory lock.LockFactory
}

func NewTeamFactory(conn Conn, lockFactory lock.LockFactory) TeamFactory {
	return &teamFactory{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (factory *teamFactory) CreateTeam(t atc.Team) (Team, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	auth, err := json.Marshal(t.Auth)
	if err != nil {
		return nil, err
	}

	row := psql.Insert("teams").
		Columns("name, basic_auth, auth").
		Values(t.Name, t.BasicAuth, auth).
		Suffix("RETURNING id, name, admin, basic_auth, auth").
		RunWith(tx).
		QueryRow()

	team := &team{
		conn:        factory.conn,
		lockFactory: factory.lockFactory,
	}
	err = scanTeam(team, row)

	if err != nil {
		return nil, err
	}

	createTableString := fmt.Sprintf(`
		CREATE TABLE team_build_events_%d ()
		INHERITS (build_events);`, team.ID())
	_, err = tx.Exec(createTableString)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return team, nil
}

func (factory *teamFactory) GetByID(teamID int) Team {
	return &team{
		id:          teamID,
		conn:        factory.conn,
		lockFactory: factory.lockFactory,
	}
}

func (factory *teamFactory) FindTeam(teamName string) (Team, bool, error) {
	team := &team{
		conn:        factory.conn,
		lockFactory: factory.lockFactory,
	}

	row := psql.Select("id, name, admin, basic_auth, auth").
		From("teams").
		Where(sq.Eq{"name": teamName}).
		RunWith(factory.conn).
		QueryRow()

	err := scanTeam(team, row)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return team, true, nil
}

func scanTeam(t *team, rows scannable) error {
	var basicAuthen, providerAuth sql.NullString

	err := rows.Scan(
		&t.id,
		&t.name,
		&t.admin,
		&basicAuthen,
		&providerAuth,
	)

	if basicAuthen.Valid {
		err = json.Unmarshal([]byte(basicAuthen.String), &t.basicAuth)

		if err != nil {
			return err
		}
	}

	if providerAuth.Valid {
		err = json.Unmarshal([]byte(providerAuth.String), &t.auth)

		if err != nil {
			return err
		}
	}

	return err
}
