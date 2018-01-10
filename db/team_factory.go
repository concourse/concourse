package db

import (
	"database/sql"
	"errors"
	"fmt"
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

var ErrDataIsEncrypted = errors.New("failed to decrypt data that is encrypted")
var ErrDataIsNotEncrypted = errors.New("failed to decrypt data that is not encrypted")

//go:generate counterfeiter . EncryptionStrategy

type EncryptionStrategy interface {
	Encrypt([]byte) (string, *string, error)
	Decrypt(string, *string) ([]byte, error)
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

	encryptedBasicAuthJSON, err := encryptedJSON(t.BasicAuth)
	if err != nil {
		return nil, err
	}

	auth, err := json.Marshal(t.Auth)
	if err != nil {
		return nil, err
	}

	es := factory.conn.EncryptionStrategy()
	encryptedAuth, nonce, err := es.Encrypt(auth)
	if err != nil {
		return nil, err
	}

	row := psql.Insert("teams").
		Columns("name, basic_auth, auth, nonce, admin").
		Values(t.Name, encryptedBasicAuthJSON, encryptedAuth, nonce, admin).
		Suffix("RETURNING id, name, admin, basic_auth, auth, nonce").
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

	row := psql.Select("id, name, admin, basic_auth, auth, nonce").
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
	rows, err := psql.Select("id, name, admin, basic_auth, auth, nonce").
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
	var basicAuth, providerAuth, nonce sql.NullString

	err := rows.Scan(
		&t.id,
		&t.name,
		&t.admin,
		&basicAuth,
		&providerAuth,
		&nonce,
	)

	if basicAuth.Valid {
		err = json.Unmarshal([]byte(basicAuth.String), &t.basicAuth)
		if err != nil {
			return err
		}
	}

	if providerAuth.Valid {
		var pAuth []byte

		es := factory.conn.EncryptionStrategy()

		var noncense *string
		if nonce.Valid {
			noncense = &nonce.String
		}

		pAuth, err = es.Decrypt(providerAuth.String, noncense)
		if err != nil {
			return err
		}

		err = json.Unmarshal(pAuth, &t.auth)
		if err != nil {
			return err
		}
	}

	return err
}
