package db

import (
	"database/sql"
	"encoding/json"

	"github.com/concourse/atc"
)

func (db *SQLDB) GetPipelineByTeamNameAndName(teamName string, pipelineName string) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT id, name, config, version, paused, team_id
		FROM pipelines
		WHERE name = $1
		AND team_id = (
				SELECT id FROM teams WHERE name = $2
			)
	`, pipelineName, teamName)

	return scanPipeline(row)
}

func (db *SQLDB) GetAllActivePipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, config, version, paused, team_id
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

func (db *SQLDB) GetConfig(teamName, pipelineName string) (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
		SELECT config, version
		FROM pipelines
		WHERE name = $1 AND team_id = (
			SELECT id
			FROM teams
			WHERE name = $2
		)
	`, pipelineName, teamName).Scan(&configBlob, &version)
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

type PipelinePausedState string

const (
	PipelinePaused   PipelinePausedState = "paused"
	PipelineUnpaused PipelinePausedState = "unpaused"
	PipelineNoChange PipelinePausedState = "nochange"
)

func (state PipelinePausedState) Bool() *bool {
	yes := true
	no := false

	switch state {
	case PipelinePaused:
		return &yes
	case PipelineUnpaused:
		return &no
	case PipelineNoChange:
		return nil
	default:
		panic("unknown pipeline state")
	}
}

func (db *SQLDB) SaveConfig(teamName string, pipelineName string, config atc.Config, from ConfigVersion, pausedState PipelinePausedState) (bool, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return false, err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var existingConfig int
	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE name = $1
		  AND team_id = (
				SELECT id FROM teams WHERE name = $2
		  )
	`, pipelineName, teamName).Scan(&existingConfig)
	if err != nil {
		return false, err
	}

	var result sql.Result

	if pausedState == PipelineNoChange {
		result, err = tx.Exec(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq')
				WHERE name = $2
					AND version = $3
					AND team_id = (
						SELECT id FROM teams WHERE name = $4
					)
			`, payload, pipelineName, from, teamName)
	} else {
		result, err = tx.Exec(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq'), paused = $2
				WHERE name = $3
					AND version = $4
					AND team_id = (
						SELECT id FROM teams WHERE name = $5
					)
			`, payload, pausedState.Bool(), pipelineName, from, teamName)
	}

	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	created := false

	if rows == 0 {
		if existingConfig == 0 {
			// If there is no state to change from then start the pipeline out as
			// paused.
			if pausedState == PipelineNoChange {
				pausedState = PipelinePaused
			}

			created = true

			_, err := tx.Exec(`
			INSERT INTO pipelines (name, config, version, ordering, paused, team_id)
			VALUES (
				$1,
				$2,
				nextval('config_version_seq'),
				(SELECT COUNT(1) + 1 FROM pipelines),
				$3,
				(SELECT id FROM teams WHERE name = $4)
			)
		`, pipelineName, payload, pausedState.Bool(), teamName)
			if err != nil {
				return false, err
			}
		} else {
			return false, ErrConfigComparisonFailed
		}
	}

	return created, tx.Commit()
}

func scanPipeline(rows scannable) (SavedPipeline, error) {
	var id int
	var name string
	var configBlob []byte
	var version int
	var paused bool
	var teamID int

	err := rows.Scan(&id, &name, &configBlob, &version, &paused, &teamID)
	if err != nil {
		return SavedPipeline{}, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedPipeline{}, err
	}

	return SavedPipeline{
		ID:     id,
		Paused: paused,
		TeamID: teamID,
		Pipeline: Pipeline{
			Name:    name,
			Config:  config,
			Version: ConfigVersion(version),
		},
	}, nil
}
