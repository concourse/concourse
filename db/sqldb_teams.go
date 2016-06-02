package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

func (db *SQLDB) CreateDefaultTeamIfNotExists() error {
	_, err := db.conn.Exec(`
	INSERT INTO teams (
    name, admin
	)
	SELECT $1, true
	WHERE NOT EXISTS (
		SELECT id FROM teams WHERE name = $1
	)
	`, atc.DefaultTeamName)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec(`
		UPDATE teams
		SET admin = true
		WHERE name = $1
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

	return db.queryTeam(fmt.Sprintf(`
	INSERT INTO teams (
    name, basic_auth, github_auth, uaa_auth
	) VALUES (
		'%s', '%s', '%s', '%s'
	)
	RETURNING id, name, admin, basic_auth, github_auth, uaa_auth
	`, team.Name, jsonEncodedBasicAuth, string(jsonEncodedGitHubAuth), string(jsonEncodedUAAAuth),
	))
}

func (db *SQLDB) queryTeam(query string) (SavedTeam, error) {
	var basicAuth, gitHubAuth, uaaAuth sql.NullString
	var savedTeam SavedTeam

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedTeam{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query).Scan(
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
	err = tx.Commit()
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
		WHERE name ILIKE $1
	`, teamName)
	return err
}
