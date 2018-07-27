package db

import (
	"database/sql"
	"strings"

	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . TeamFactory

type TeamFactory interface {
	CreateTeam(atc.Team) (Team, error)
	FindTeam(string) (Team, bool, error)
	GetTeams() ([]Team, error)
	GetByID(teamID int) Team
	CreateDefaultTeamIfNotExists() (Team, error)
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
	return factory.createTeam(t, false)
}

func (factory *teamFactory) createTeam(t atc.Team, admin bool) (Team, error) {
	tx, err := factory.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	auth, err := json.Marshal(t.Auth)
	if err != nil {
		return nil, err
	}

	row := psql.Insert("teams").
		Columns("name, auth, admin").
		Values(t.Name, auth, admin).
		Suffix("RETURNING id, name, admin, auth").
		RunWith(tx).
		QueryRow()

	team := &team{
		conn:        factory.conn,
		lockFactory: factory.lockFactory,
	}
	err = factory.scanTeam(team, row)

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

	row := psql.Select("id, name, admin, auth").
		From("teams").
		Where(sq.Eq{"LOWER(name)": strings.ToLower(teamName)}).
		RunWith(factory.conn).
		QueryRow()

	err := factory.scanTeam(team, row)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return team, true, nil
}

func (factory *teamFactory) GetTeams() ([]Team, error) {
	rows, err := psql.Select("id, name, admin, auth").
		From("teams").
		OrderBy("id ASC").
		RunWith(factory.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	teams := []Team{}

	for rows.Next() {
		team := &team{
			conn:        factory.conn,
			lockFactory: factory.lockFactory,
		}

		err = factory.scanTeam(team, rows)
		if err != nil {
			return nil, err
		}

		teams = append(teams, team)
	}

	return teams, nil
}

func (factory *teamFactory) CreateDefaultTeamIfNotExists() (Team, error) {
	_, err := psql.Update("teams").
		Set("admin", true).
		Where(sq.Eq{"LOWER(name)": strings.ToLower(atc.DefaultTeamName)}).
		RunWith(factory.conn).
		Exec()

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	t, found, err := factory.FindTeam(atc.DefaultTeamName)
	if err != nil {
		return nil, err
	}

	if found {
		return t, nil
	}

	//not found, have to create
	return factory.createTeam(atc.Team{
		Name: atc.DefaultTeamName,
	},
		true,
	)
}

func (factory *teamFactory) scanTeam(t *team, rows scannable) error {
	var providerAuth sql.NullString

	err := rows.Scan(
		&t.id,
		&t.name,
		&t.admin,
		&providerAuth,
	)

	if providerAuth.Valid {
		err = json.Unmarshal([]byte(providerAuth.String), &t.auth)
		if err != nil {
			return err
		}
	}

	return err
}
