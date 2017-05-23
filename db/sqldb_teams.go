package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/concourse/atc"
)

func (db *SQLDB) CreateDefaultTeamIfNotExists() error {
	var id sql.NullInt64
	err := db.conn.QueryRow(`
			SELECT id
			FROM teams
			WHERE name = $1
		`, atc.DefaultTeamName).Scan(&id)

	if err != nil {
		if err == sql.ErrNoRows {
			err = db.conn.QueryRow(`
				INSERT INTO teams (
					name, admin
				)
				VALUES ($1, true)
				RETURNING id
			`, atc.DefaultTeamName).Scan(&id)
			if err != nil {
				return err
			}

			if !id.Valid {
				return errors.New("could-not-unmarshal-id")
			}
			createTableString := fmt.Sprintf(`
						CREATE TABLE team_build_events_%d ()
						INHERITS (build_events);`, id.Int64)
			_, err = db.conn.Exec(createTableString)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		_, err = db.conn.Exec(`
			UPDATE teams
			SET admin = true
			WHERE LOWER(name) = LOWER($1)
		`, atc.DefaultTeamName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *SQLDB) CreateTeam(team Team) (SavedTeam, error) {
	savedTeam, err := scanTeam(db.conn.QueryRow(`
	INSERT INTO teams (
    name
	) VALUES (
		$1
	)
	RETURNING id, name, admin
	`, team.Name))
	if err != nil {
		return SavedTeam{}, err
	}

	createTableString := fmt.Sprintf(`
		CREATE TABLE team_build_events_%d ()
		INHERITS (build_events);`, savedTeam.ID)
	_, err = db.conn.Exec(createTableString)
	if err != nil {
		return SavedTeam{}, err
	}

	return savedTeam, nil
}

func scanTeam(rows scannable) (SavedTeam, error) {
	var savedTeam SavedTeam

	err := rows.Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
	)
	if err != nil {
		return savedTeam, err
	}

	return savedTeam, nil
}
