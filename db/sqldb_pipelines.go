package db

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc"
)

const pipelineColumns = "id, name, config, version, paused, team_id"

func (db *SQLDB) GetPipelineByID(pipelineID int) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines
		WHERE id = $1
	`, pipelineID)

	return scanPipeline(row)
}

func (db *SQLDB) GetAllPipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT ` + pipelineColumns + `
		FROM pipelines
		ORDER BY ordering
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	pipelines := []SavedPipeline{}

	for rows.Next() {
		pipeline, err := scanPipeline(rows)

		if err != nil {
			return nil, err
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

func (db *SQLDB) OrderPipelines(pipelineNames []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var pipelineCount int

	err = tx.QueryRow(`
			SELECT COUNT(1)
			FROM pipelines
	`).Scan(&pipelineCount)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET ordering = $1
	`, pipelineCount+1)

	if err != nil {
		return err
	}

	for i, name := range pipelineNames {
		_, err = tx.Exec(`
			UPDATE pipelines
			SET ordering = $1
			WHERE name = $2
		`, i, name)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *SQLDB) GetConfigByBuildID(buildID int) (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
			SELECT p.config, p.version
			FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			WHERE b.ID = $1
		`, buildID).Scan(&configBlob, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, 0, nil
		} else {
			return atc.Config{}, 0, err
		}
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, 0, err
	}

	return config, ConfigVersion(version), nil
}
