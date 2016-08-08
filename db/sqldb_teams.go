package db

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc"
)

func (db *SQLDB) GetTeams() ([]SavedTeam, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, admin, basic_auth, github_auth, uaa_auth FROM teams
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	teams := []SavedTeam{}

	for rows.Next() {
		team, err := scanTeam(rows)

		if err != nil {
			return nil, err
		}

		teams = append(teams, team)
	}

	return teams, nil
}

func (db *SQLDB) CreateDefaultTeamIfNotExists() error {
	_, err := db.conn.Exec(`
	INSERT INTO teams (
    name, admin
	)
	SELECT $1, true
	WHERE NOT EXISTS (
		SELECT id FROM teams WHERE LOWER(name) = LOWER($1)
	)
	`, atc.DefaultTeamName)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		UPDATE teams
		SET admin = true
		WHERE LOWER(name) = LOWER($1)
	`, atc.DefaultTeamName)
	return err
}

func (db *SQLDB) CreateTeam(team Team) (SavedTeam, error) {
	jsonEncodedBasicAuth, err := team.BasicAuth.EncryptedJSON()
	if err != nil {
		return SavedTeam{}, err
	}

	var gitHubAuth *GitHubAuth
	if team.GitHubAuth != nil && team.GitHubAuth.ClientID != "" && team.GitHubAuth.ClientSecret != "" {
		gitHubAuth = team.GitHubAuth
	}
	jsonEncodedGitHubAuth, err := json.Marshal(gitHubAuth)
	if err != nil {
		return SavedTeam{}, err
	}

	jsonEncodedUAAAuth, err := json.Marshal(team.UAAAuth)
	if err != nil {
		return SavedTeam{}, err
	}
	return scanTeam(db.conn.QueryRow(`
	INSERT INTO teams (
    name, basic_auth, github_auth, uaa_auth
	) VALUES (
		$1, $2, $3, $4
	)
	RETURNING id, name, admin, basic_auth, github_auth, uaa_auth
	`, team.Name, jsonEncodedBasicAuth, string(jsonEncodedGitHubAuth), string(jsonEncodedUAAAuth)))
}

func scanTeam(rows scannable) (SavedTeam, error) {
	var basicAuth, gitHubAuth, uaaAuth sql.NullString
	var savedTeam SavedTeam

	err := rows.Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
		&basicAuth,
		&gitHubAuth,
		&uaaAuth,
	)
	if err != nil {
		return savedTeam, err
	}

	if basicAuth.Valid {
		err = json.Unmarshal([]byte(basicAuth.String), &savedTeam.BasicAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if gitHubAuth.Valid {
		err = json.Unmarshal([]byte(gitHubAuth.String), &savedTeam.GitHubAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	if uaaAuth.Valid {
		err = json.Unmarshal([]byte(uaaAuth.String), &savedTeam.UAAAuth)
		if err != nil {
			return savedTeam, err
		}
	}

	return savedTeam, nil
}

func (db *SQLDB) DeleteTeamByName(teamName string) error {
	_, err := db.conn.Exec(`
    DELETE FROM teams
		WHERE LOWER(name) = LOWER($1)
	`, teamName)
	return err
}
