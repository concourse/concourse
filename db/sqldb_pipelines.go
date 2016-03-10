package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

const pipelineColumns = "id, name, config, version, paused, team_id"

func (db *SQLDB) GetPipelineByTeamNameAndName(teamName string, pipelineName string) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines
		WHERE name = $1
		AND team_id = (
				SELECT id FROM teams WHERE name = $2
			)
	`, pipelineName, teamName)

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
		}
		return atc.Config{}, 0, err
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

func (db *SQLDB) SaveConfig(
	teamName string, pipelineName string, config atc.Config, from ConfigVersion, pausedState PipelinePausedState,
) (SavedPipeline, bool, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return SavedPipeline{}, false, err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedPipeline{}, false, err
	}

	defer tx.Rollback()

	var created bool
	var savedPipeline SavedPipeline

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
		return SavedPipeline{}, false, err
	}

	if existingConfig == 0 {
		if pausedState == PipelineNoChange {
			pausedState = PipelinePaused
		}

		savedPipeline, err = scanPipeline(tx.QueryRow(`
		INSERT INTO pipelines (name, config, version, ordering, paused, team_id)
		VALUES (
			$1,
			$2,
			nextval('config_version_seq'),
			(SELECT COUNT(1) + 1 FROM pipelines),
			$3,
			(SELECT id FROM teams WHERE name = $4)
		)
		RETURNING `+pipelineColumns+`
		`, pipelineName, payload, pausedState.Bool(), teamName))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE TABLE pipeline_build_events_%[1]d ()
		INHERITS (build_events);

		CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id);

		CREATE UNIQUE INDEX pipeline_build_events_%[1]d_build_id_event_id ON pipeline_build_events_%[1]d (build_id, event_id);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
		}
	} else {
		if pausedState == PipelineNoChange {
			savedPipeline, err = scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq')
			WHERE name = $2
			AND version = $3
			AND team_id = (
				SELECT id FROM teams WHERE name = $4
			)
			RETURNING `+pipelineColumns+`
			`, payload, pipelineName, from, teamName))
		} else {
			savedPipeline, err = scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq'), paused = $2
			WHERE name = $3
			AND version = $4
			AND team_id = (
				SELECT id FROM teams WHERE name = $5
			)
			RETURNING `+pipelineColumns+`
			`, payload, pausedState.Bool(), pipelineName, from, teamName))
		}

		if err != nil && err != sql.ErrNoRows {
			return SavedPipeline{}, false, err
		}

		if savedPipeline.ID == 0 {
			return SavedPipeline{}, false, ErrConfigComparisonFailed
		}

		_, err = tx.Exec(`
      DELETE FROM jobs_serial_groups
      WHERE job_id in (
        SELECT j.id
        FROM jobs j
        WHERE j.pipeline_id = $1
      )
		`, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resource := range config.Resources {
		err = db.registerResource(tx, resource.Name, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, job := range config.Jobs {
		err = db.registerJob(tx, job.Name, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}

		for _, sg := range job.SerialGroups {
			err = db.registerSerialGroup(tx, job.Name, sg, savedPipeline.ID)
			if err != nil {
				return SavedPipeline{}, false, err
			}
		}
	}

	return savedPipeline, created, tx.Commit()
}

func (db *SQLDB) registerJob(tx Tx, name string, pipelineID int) error {
	_, err := tx.Exec(`
		INSERT INTO jobs (name, pipeline_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs WHERE name = $1 AND pipeline_id = $2
		)
	`, name, pipelineID)

	return swallowUniqueViolation(err)
}

func (db *SQLDB) registerSerialGroup(tx Tx, jobName, serialGroup string, pipelineID int) error {
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

func (db *SQLDB) registerResource(tx Tx, name string, pipelineID int) error {
	_, err := tx.Exec(`
		INSERT INTO resources (name, pipeline_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM resources WHERE name = $1 AND pipeline_id = $2
		)
	`, name, pipelineID)

	return swallowUniqueViolation(err)
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
