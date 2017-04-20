package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/concourse/atc"
)

//go:generate counterfeiter . TeamDB

type TeamDB interface {
	GetPipelineByName(pipelineName string) (SavedPipeline, bool, error)

	GetTeam() (SavedTeam, bool, error)
	GetConfig(pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error)
	SaveConfigToBeDeprecated(string, atc.Config, ConfigVersion, PipelinePausedState) (SavedPipeline, bool, error)

	CreateOneOffBuild() (Build, error)
}

type teamDB struct {
	teamName string

	conn         Conn
	buildFactory *buildFactory
}

func (db *teamDB) GetPipelineByName(pipelineName string) (SavedPipeline, bool, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.name = $1
		AND p.team_id = (
			SELECT id FROM teams WHERE LOWER(name) = LOWER($2)
		)
	`, pipelineName, db.teamName)
	pipeline, err := scanPipeline(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedPipeline{}, false, nil
		}

		return SavedPipeline{}, false, err
	}

	return pipeline, true, nil
}

func (db *teamDB) GetConfig(pipelineName string) (atc.Config, atc.RawConfig, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
		SELECT config, version
		FROM pipelines
		WHERE name = $1 AND team_id = (
			SELECT id
			FROM teams
			WHERE LOWER(name) = LOWER($2)
		)
	`, pipelineName, db.teamName).Scan(&configBlob, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, atc.RawConfig(""), 0, nil
		}
		return atc.Config{}, atc.RawConfig(""), 0, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, atc.RawConfig(string(configBlob)), ConfigVersion(version), atc.MalformedConfigError{err}
	}

	return config, atc.RawConfig(string(configBlob)), ConfigVersion(version), nil
}

// only used for tests in db package, use dbng.Team.SavePipeline instead
func (db *teamDB) SaveConfigToBeDeprecated(
	pipelineName string,
	config atc.Config,
	from ConfigVersion,
	pausedState PipelinePausedState,
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

	var teamID int
	err = tx.QueryRow(`SELECT id FROM teams WHERE LOWER(name) = LOWER($1)`, db.teamName).Scan(&teamID)
	if err != nil {
		return SavedPipeline{}, false, err
	}

	var created bool
	var savedPipeline SavedPipeline

	var existingConfig int
	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE name = $1
	  AND team_id = $2
	`, pipelineName, teamID).Scan(&existingConfig)
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
			$4
		)
		RETURNING `+unqualifiedPipelineColumns+`,
		(
			SELECT t.name as team_name FROM teams t WHERE t.id = $4
		)
		`, pipelineName, payload, pausedState.Bool(), teamID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		created = true

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE TABLE pipeline_build_events_%[1]d ()
		INHERITS (build_events);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
		CREATE INDEX pipeline_build_events_%[1]d_build_id ON pipeline_build_events_%[1]d (build_id);
		`, savedPipeline.ID))
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(fmt.Sprintf(`
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
			AND team_id = $4
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pipelineName, from, teamID))
		} else {
			savedPipeline, err = scanPipeline(tx.QueryRow(`
			UPDATE pipelines
			SET config = $1, version = nextval('config_version_seq'), paused = $2
			WHERE name = $3
			AND version = $4
			AND team_id = $5
			RETURNING `+unqualifiedPipelineColumns+`,
			(
				SELECT t.name as team_name FROM teams t WHERE t.id = $4
			)
			`, payload, pausedState.Bool(), pipelineName, from, teamID))
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

		_, err = tx.Exec(`
			UPDATE jobs
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(`
			UPDATE resources
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}

		_, err = tx.Exec(`
			UPDATE resource_types
			SET active = false
			WHERE pipeline_id = $1
		`, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resource := range config.Resources {
		err = db.saveResource(tx, resource, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, resourceType := range config.ResourceTypes {
		err = db.saveResourceType(tx, resourceType, savedPipeline.ID)
		if err != nil {
			return SavedPipeline{}, false, err
		}
	}

	for _, job := range config.Jobs {
		err = db.saveJob(tx, job, savedPipeline.ID)
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

func (db *teamDB) saveJob(tx Tx, job atc.JobConfig, pipelineID int) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE jobs
		SET config = $3, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, job.Name, pipelineID, configPayload)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO jobs (name, pipeline_id, config, active)
		VALUES ($1, $2, $3, true)
	`, job.Name, pipelineID, configPayload)

	return swallowUniqueViolation(err)
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

func (db *teamDB) saveResource(tx Tx, resource atc.ResourceConfig, pipelineID int) error {
	configPayload, err := json.Marshal(resource)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET config = $3, source_hash = $4, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, resource.Name, pipelineID, configPayload, mapHash(resource.Source))
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resources (name, pipeline_id, config, source_hash, active)
		VALUES ($1, $2, $3, $4, true)
	`, resource.Name, pipelineID, configPayload, mapHash(resource.Source))

	return swallowUniqueViolation(err)
}

func (db *teamDB) saveResourceType(tx Tx, resourceType atc.ResourceType, pipelineID int) error {
	configPayload, err := json.Marshal(resourceType)
	if err != nil {
		return err
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resource_types
		SET config = $3, type = $4, active = true
		WHERE name = $1 AND pipeline_id = $2
	`, resourceType.Name, pipelineID, configPayload, resourceType.Type)
	if err != nil {
		return err
	}

	if updated {
		return nil
	}

	_, err = tx.Exec(`
		INSERT INTO resource_types (name, type, pipeline_id, config, active)
		VALUES ($1, $2, $3, $4, true)
	`, resourceType.Name, resourceType.Type, pipelineID, configPayload)

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

func (db *teamDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	build, _, err := db.buildFactory.ScanBuild(tx.QueryRow(`
		INSERT INTO builds (name, team_id, status)
		SELECT nextval('one_off_name'), t.id, 'pending'
		FROM teams t WHERE LOWER(t.name) = LOWER($1)
		RETURNING `+buildColumns+`, null, null, null,
		(
			SELECT name FROM teams WHERE LOWER(name) = LOWER($1)
		)
	`, string(db.teamName)))
	if err != nil {
		return nil, err
	}

	err = createBuildEventSeq(tx, build.ID())
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func scanPipeline(rows scannable) (SavedPipeline, error) {
	var id int
	var name string
	var configBlob []byte
	var version int
	var paused bool
	var public bool
	var teamID int
	var teamName string

	err := rows.Scan(&id, &name, &configBlob, &version, &paused, &teamID, &public, &teamName)
	if err != nil {
		return SavedPipeline{}, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedPipeline{}, err
	}

	return SavedPipeline{
		ID:       id,
		Paused:   paused,
		Public:   public,
		TeamID:   teamID,
		TeamName: teamName,
		Pipeline: Pipeline{
			Name:    name,
			Config:  config,
			Version: ConfigVersion(version),
		},
	}, nil
}

func scanPipelines(rows *sql.Rows) ([]SavedPipeline, error) {
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

func mapHash(m map[string]interface{}) string {
	j, _ := json.Marshal(m)
	return fmt.Sprintf("%x", sha256.Sum256(j))
}
