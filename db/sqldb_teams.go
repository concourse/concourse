package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"golang.org/x/crypto/bcrypt"

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

func (db *SQLDB) CreateTeam(data Team) (SavedTeam, error) {
	jsonEncodedBasicAuth, err := db.jsonEncodeTeamBasicAuth(data)
	if err != nil {
		return SavedTeam{}, err
	}
	jsonEncodedGitHubAuth, err := db.jsonEncodeTeamGitHubAuth(data)
	if err != nil {
		return SavedTeam{}, err
	}

	return db.queryTeam(fmt.Sprintf(`
	INSERT INTO teams (
    name, basic_auth, github_auth
	) VALUES (
		'%s', '%s', '%s'
	)
	RETURNING id, name, admin, basic_auth, github_auth
	`, data.Name, jsonEncodedBasicAuth, jsonEncodedGitHubAuth,
	))
}

func (db *SQLDB) queryTeam(query string) (SavedTeam, error) {
	var basicAuth, gitHubAuth sql.NullString
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

	return savedTeam, nil
}

func (db *SQLDB) jsonEncodeTeamGitHubAuth(team Team) (string, error) {
	if team.ClientID == "" || team.ClientSecret == "" {
		team.GitHubAuth = GitHubAuth{}
	}

	json, err := json.Marshal(team.GitHubAuth)
	return string(json), err
}

func (db *SQLDB) UpdateTeamGitHubAuth(team Team) (SavedTeam, error) {
	gitHubAuth, err := db.jsonEncodeTeamGitHubAuth(team)
	if err != nil {
		return SavedTeam{}, err
	}

	query := fmt.Sprintf(`
		UPDATE teams
		SET github_auth = '%s'
		WHERE name ILIKE '%s'
		RETURNING id, name, admin, basic_auth, github_auth
	`, gitHubAuth, team.Name,
	)
	return db.queryTeam(query)
}

func (db *SQLDB) jsonEncodeTeamBasicAuth(team Team) (string, error) {
	if team.BasicAuthUsername == "" || team.BasicAuthPassword == "" {
		team.BasicAuth = BasicAuth{}
	} else {
		encryptedPw, err := bcrypt.GenerateFromPassword([]byte(team.BasicAuthPassword), 4)
		if err != nil {
			return "", err
		}
		team.BasicAuthPassword = string(encryptedPw)
	}

	json, err := json.Marshal(team.BasicAuth)
	return string(json), err
}

func (db *SQLDB) UpdateTeamBasicAuth(team Team) (SavedTeam, error) {
	basicAuth, err := db.jsonEncodeTeamBasicAuth(team)
	if err != nil {
		return SavedTeam{}, err
	}

	query := fmt.Sprintf(`
		UPDATE teams
		SET basic_auth = '%s'
		WHERE name ILIKE '%s'
		RETURNING id, name, admin, basic_auth, github_auth
	`, basicAuth, team.Name)
	return db.queryTeam(query)
}

func (db *SQLDB) DeleteTeamByName(teamName string) error {
	_, err := db.conn.Exec(`
    DELETE FROM teams
		WHERE name ILIKE $1
	`, teamName)
	return err
}
