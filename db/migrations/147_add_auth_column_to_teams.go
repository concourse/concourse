package migrations

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc/db/migration"
)

func AddAuthToTeams(tx migration.LimitedTx) error {
	_, err := tx.Exec(`
    ALTER TABLE teams
    ADD COLUMN auth json NULL;
	`)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		SELECT id, github_auth, uaa_auth, genericoauth_auth
		FROM teams
	`)

	if err != nil {
		return err
	}

	defer rows.Close()

	teamConfigs := map[int][]byte{}

	for rows.Next() {
		var (
			id int

			githubAuth, uaaAuth, genericOAuth sql.NullString
		)

		err := rows.Scan(&id, &githubAuth, &uaaAuth, &genericOAuth)
		if err != nil {
			return err
		}

		authConfigs := make(map[string]*json.RawMessage)

		if githubAuth.Valid && githubAuth.String != "null" {
			data := []byte(githubAuth.String)
			authConfigs["github"] = (*json.RawMessage)(&data)
		}

		if uaaAuth.Valid && uaaAuth.String != "null" {
			data := []byte(uaaAuth.String)
			authConfigs["uaa"] = (*json.RawMessage)(&data)
		}

		if genericOAuth.Valid && genericOAuth.String != "null" {
			data := []byte(genericOAuth.String)
			authConfigs["oauth"] = (*json.RawMessage)(&data)
		}

		jsonConfig, err := json.Marshal(authConfigs)
		if err != nil {
			return err
		}

		teamConfigs[id] = jsonConfig
	}

	for id, jsonConfig := range teamConfigs {
		_, err = tx.Exec(`
			UPDATE teams
			SET auth = $1
			WHERE id = $2
		`, jsonConfig, id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		ALTER TABLE teams DROP COLUMN github_auth, DROP COLUMN uaa_auth, DROP COLUMN genericoauth_auth;
	`)
	if err != nil {
		return err
	}

	return nil
}
