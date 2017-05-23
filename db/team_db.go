package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
)

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	GetTeam() (SavedTeam, bool, error)
}

type teamDB struct {
	teamName string

	conn Conn
}

func (db *teamDB) registerSerialGroup(tx Tx, jobName, serialGroup string, pipelineID int) error {
	_, err := tx.Exec(`
    INSERT INTO jobs_serial_groups (serial_group, job_id) VALUES
    ($1, (SELECT j.id
                  FROM jobs j
                       JOIN pipelines p
                         ON j.pipeline_id = p.id
                  WHERE j.name = $2
                    AND j.pipeline_id = $3
                 LIMIT  1));`,
		serialGroup, jobName, pipelineID,
	)

	return swallowUniqueViolation(err)
}

func (db *teamDB) GetTeam() (SavedTeam, bool, error) {
	query := `
		SELECT id, name, admin
		FROM teams
		WHERE LOWER(name) = LOWER($1)
	`
	params := []interface{}{db.teamName}
	savedTeam, err := db.queryTeam(query, params)
	if err != nil {
		if err == sql.ErrNoRows {
			return savedTeam, false, nil
		}

		return savedTeam, false, err
	}

	return savedTeam, true, nil
}

func (db *teamDB) queryTeam(query string, params []interface{}) (SavedTeam, error) {
	var savedTeam SavedTeam

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedTeam{}, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(query, params...).Scan(
		&savedTeam.ID,
		&savedTeam.Name,
		&savedTeam.Admin,
	)
	if err != nil {
		return savedTeam, err
	}
	err = tx.Commit()
	if err != nil {
		return savedTeam, err
	}

	return savedTeam, nil
}

func mapHash(m map[string]interface{}) string {
	j, _ := json.Marshal(m)
	return fmt.Sprintf("%x", sha256.Sum256(j))
}
