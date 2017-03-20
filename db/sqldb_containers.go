package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/concourse/atc"
)

const containerColumns = "c.worker_name, resource_id, check_type, check_source, build_id, plan_id, stage, handle, b.name as build_name, r.name as resource_name, p.id as pipeline_id, p.name as pipeline_name, j.name as job_name, step_name, type, working_directory, env_variables, attempts, process_user, c.id, resource_type_version, c.team_id"

const containerJoins = `
		LEFT JOIN pipelines p
		  ON p.id = c.pipeline_id
		LEFT JOIN resources r
			ON r.id = c.resource_id
		LEFT JOIN builds b
		  ON b.id = c.build_id
		LEFT JOIN jobs j
		  ON j.id = b.job_id`

var ErrInvalidIdentifier = errors.New("invalid container identifier")

func scanRows(rows *sql.Rows) ([]SavedContainer, error) {
	var containers []SavedContainer
	for rows.Next() {
		container, err := scanContainer(rows)
		if err != nil {
			return nil, nil
		}
		containers = append(containers, container)
	}

	return containers, nil
}

func (db *SQLDB) GetContainer(handle string) (SavedContainer, bool, error) {
	container, err := scanContainer(db.conn.QueryRow(`
		SELECT `+containerColumns+`
	  FROM containers c `+containerJoins+`
		WHERE c.handle = $1
	`, handle))

	if err != nil {
		if err == sql.ErrNoRows {
			return SavedContainer{}, false, nil
		}
		return SavedContainer{}, false, err
	}

	return container, true, nil
}

// this saves off metadata and other things not yet expressed by dbng (best_if_used_by)
func (db *SQLDB) PutTheRestOfThisCrapInTheDatabaseButPleaseRemoveMeLater(container Container, maxLifetime time.Duration) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	maxLifetimeValue := "NULL"
	if maxLifetime > 0 {
		maxLifetimeValue = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(maxLifetime.Seconds()))
	}

	var imageResourceSource sql.NullString
	if container.ImageResourceSource != nil {
		marshaled, err := json.Marshal(container.ImageResourceSource)
		if err != nil {
			return err
		}

		imageResourceSource.String = string(marshaled)
		imageResourceSource.Valid = true
	}

	var imageResourceType sql.NullString
	if container.ImageResourceType != "" {
		imageResourceType.String = container.ImageResourceType
		imageResourceType.Valid = true
	}

	var attempts sql.NullString
	if len(container.Attempts) > 0 {
		attemptsBlob, err := json.Marshal(container.Attempts)
		if err != nil {
			return err
		}
		attempts.Valid = true
		attempts.String = string(attemptsBlob)
	}

	var id int
	err = tx.QueryRow(`
		UPDATE containers SET (
			best_if_used_by,
			image_resource_type,
			image_resource_source,
			process_user,
			attempts,
			pipeline_id
		) = (
			`+maxLifetimeValue+`,
			$2,
			$3,
			$4,
			$5,
			$6
		)
		WHERE handle = $1
		RETURNING id`,
		container.Handle,
		imageResourceType,
		imageResourceSource,
		container.User,
		attempts,
		container.PipelineID,
	).Scan(&id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func scanContainer(row scannable) (SavedContainer, error) {
	var (
		teamID              sql.NullInt64
		resourceID          sql.NullInt64
		checkSourceBlob     []byte
		buildID             sql.NullInt64
		planID              sql.NullString
		stage               string
		buildName           sql.NullString
		resourceName        sql.NullString
		pipelineID          sql.NullInt64
		pipelineName        sql.NullString
		jobName             sql.NullString
		infoType            string
		envVariablesBlob    []byte
		attempts            sql.NullString
		checkType           sql.NullString
		user                sql.NullString
		resourceTypeVersion []byte
	)
	container := SavedContainer{}

	err := row.Scan(
		&container.WorkerName,
		&resourceID,
		&checkType,
		&checkSourceBlob,
		&buildID,
		&planID,
		&stage,
		&container.Handle,
		&buildName,
		&resourceName,
		&pipelineID,
		&pipelineName,
		&jobName,
		&container.StepName,
		&infoType,
		&container.WorkingDirectory,
		&envVariablesBlob,
		&attempts,
		&user,
		&container.ID,
		&resourceTypeVersion,
		&teamID,
	)

	if err != nil {
		return SavedContainer{}, err
	}

	if resourceID.Valid {
		container.ResourceID = int(resourceID.Int64)
	}

	if buildID.Valid {
		container.ContainerIdentifier.BuildID = int(buildID.Int64)
	}

	if teamID.Valid {
		container.TeamID = int(teamID.Int64)
	}

	if user.Valid {
		container.User = user.String
	}

	container.PlanID = atc.PlanID(planID.String)

	container.Stage = ContainerStage(stage)

	if buildName.Valid {
		container.BuildName = buildName.String
	}

	if resourceName.Valid {
		container.ResourceName = resourceName.String
	}

	if pipelineID.Valid {
		container.PipelineID = int(pipelineID.Int64)
	}

	if pipelineName.Valid {
		container.PipelineName = pipelineName.String
	}

	if jobName.Valid {
		container.JobName = jobName.String
	}

	container.Type, err = ContainerTypeFromString(infoType)
	if err != nil {
		return SavedContainer{}, err
	}

	err = json.Unmarshal(envVariablesBlob, &container.EnvironmentVariables)
	if err != nil {
		return SavedContainer{}, err
	}

	if attempts.Valid {
		err = json.Unmarshal([]byte(attempts.String), &container.Attempts)
		if err != nil {
			return SavedContainer{}, err
		}
	}

	//TODO remove this check once all containers have a user
	// specifically waiting upon worker provided resources to
	// use image resources that specifiy a metadata.json with
	// a user
	if container.User == "" {
		container.User = "root"
	}

	return container, nil
}
