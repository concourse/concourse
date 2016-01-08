package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/concourse/atc"
)

const containerMetadataColumns = "handle, expires_at, b.id as build_id, b.name as build_name, r.name as resource_name, worker_name, p.id as pipeline_id, p.name as pipeline_name, j.name as job_name, step_name, type, working_directory, check_type, check_source, env_variables, attempts"
const containerIdentifierColumns = "handle, expires_at, worker_name, resource_id, build_id, plan_id"

func (db *SQLDB) FindContainersByMetadata(id ContainerMetadata) ([]Container, error) {
	err := deleteExpired(db)
	if err != nil {
		return nil, err
	}

	whereCriteria := []string{}
	params := []interface{}{}

	if id.ResourceName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("r.name = $%d", len(params)+1))
		params = append(params, id.ResourceName)
	}

	if id.StepName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("c.step_name = $%d", len(params)+1))
		params = append(params, id.StepName)
	}

	if id.JobName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("j.name = $%d", len(params)+1))
		params = append(params, id.JobName)
	}

	if id.PipelineName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("p.name = $%d", len(params)+1))
		params = append(params, id.PipelineName)
	}

	if id.BuildID != 0 {
		whereCriteria = append(whereCriteria, fmt.Sprintf("build_id = $%d", len(params)+1))
		params = append(params, id.BuildID)
	}

	if id.Type != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("type = $%d", len(params)+1))
		params = append(params, id.Type.String())
	}

	if id.WorkerName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("worker_name = $%d", len(params)+1))
		params = append(params, id.WorkerName)
	}

	if id.CheckType != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_type = $%d", len(params)+1))
		params = append(params, id.CheckType)
	}

	var checkSourceBlob []byte
	if id.CheckSource != nil {
		checkSourceBlob, err = json.Marshal(id.CheckSource)
		if err != nil {
			return nil, err
		}
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_source = $%d", len(params)+1))
		params = append(params, checkSourceBlob)
	}

	var rows *sql.Rows
	selectQuery := `
		SELECT ` + containerMetadataColumns + `
		FROM containers c
		LEFT JOIN pipelines p
		  ON p.id = c.pipeline_id
		LEFT JOIN resources r
		  ON r.id = c.resource_id
	  LEFT JOIN builds b
		  ON b.id = c.build_id
		LEFT JOIN jobs j
		  ON j.id = b.job_id
	  	`
	if len(whereCriteria) > 0 {
		selectQuery += fmt.Sprintf("WHERE %s", strings.Join(whereCriteria, " AND "))
	}

	rows, err = db.conn.Query(selectQuery, params...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	infos := []Container{}
	for rows.Next() {
		info, err := scanContainerMetadata(rows)

		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func (db *SQLDB) FindContainerByIdentifier(id ContainerIdentifier) (Container, bool, error) {
	err := deleteExpired(db)
	if err != nil {
		return Container{}, false, err
	}

	var containers []Container
	var selectQuery string
	var rows *sql.Rows
	if id.ResourceID != 0 {
		selectQuery = `
			SELECT ` + containerIdentifierColumns + `
	  	FROM containers
		  WHERE resource_id = $1
	  	`
		rows, err = db.conn.Query(selectQuery, id.ResourceID)
	} else if id.BuildID != 0 && id.PlanID != "" {
		selectQuery = `
			SELECT ` + containerIdentifierColumns + `
	    FROM containers
		  WHERE build_id = $1 AND plan_id = $2
	  	`
		rows, err = db.conn.Query(selectQuery, id.BuildID, string(id.PlanID))
	} else {
		return Container{}, false, errors.New("insufficient container identifiers")
	}

	if err != nil {
		return Container{}, false, err
	}

	for rows.Next() {
		container, err := scanContainerIdentifier(rows)
		if err != nil {
			return Container{}, false, nil
		}
		containers = append(containers, container)
	}

	switch len(containers) {
	case 0:
		return Container{}, false, nil

	case 1:
		return containers[0], true, nil

	default:
		return Container{}, false, ErrMultipleContainersFound
	}
}

func (db *SQLDB) GetContainer(handle string) (Container, bool, error) {
	containerWithMetadata, err := scanContainerMetadata(db.conn.QueryRow(`
		SELECT `+containerMetadataColumns+`
		FROM containers c
		LEFT JOIN pipelines p
		  ON p.id = c.pipeline_id
		LEFT JOIN resources r
			ON r.id = c.resource_id
		LEFT JOIN builds b
		  ON b.id = c.build_id
		LEFT JOIN jobs j
		  ON j.id = b.job_id
		WHERE c.handle = $1
	`, handle))

	if err != nil {
		if err == sql.ErrNoRows {
			return Container{}, false, nil
		}
		return Container{}, false, err
	}

	container, err := scanContainerIdentifier(db.conn.QueryRow(`
		SELECT `+containerIdentifierColumns+`
		FROM containers c
		WHERE c.handle = $1
	`, handle))
	if err != nil {
		if err == sql.ErrNoRows {
			return Container{}, false, nil
		}
		return Container{}, false, err
	}

	container.ContainerMetadata = containerWithMetadata.ContainerMetadata
	return container, true, nil
}

func (db *SQLDB) CreateContainer(container Container, ttl time.Duration) (Container, error) {
	tx, err := db.conn.Begin()

	if err != nil {
		return Container{}, err
	}

	checkSource, err := json.Marshal(container.CheckSource)
	if err != nil {
		return Container{}, err
	}

	envVariables, err := json.Marshal(container.EnvironmentVariables)
	if err != nil {
		return Container{}, err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	var pipelineID sql.NullInt64
	if container.PipelineName != "" {
		pipeline, err := db.GetPipelineByTeamNameAndName(atc.DefaultTeamName, container.PipelineName)
		if err != nil {
			return Container{}, fmt.Errorf("failed to find pipeline: %s", err.Error())
		}
		pipelineID.Int64 = int64(pipeline.ID)
		pipelineID.Valid = true
	}

	var resourceID sql.NullInt64
	if container.ResourceID != 0 {
		resourceID.Int64 = int64(container.ResourceID)
		resourceID.Valid = true
	}

	var buildID sql.NullInt64
	if container.ContainerIdentifier.BuildID != 0 {
		buildID.Int64 = int64(container.ContainerIdentifier.BuildID)
		buildID.Valid = true
	}

	workerName := container.ContainerMetadata.WorkerName
	if workerName == "" {
		workerName = container.ContainerIdentifier.WorkerName
	}

	var attempts string
	for _, attemptNumber := range container.Attempts {
		attemptInt := strconv.Itoa(attemptNumber)

		if attempts == "" {
			attempts = attemptInt
		} else {
			attempts = attempts + "," + attemptInt
		}
	}

	defer tx.Rollback()

	newContainer, err := scanContainerIdentifier(tx.QueryRow(`
		INSERT INTO containers (handle, resource_id, step_name, pipeline_id, build_id, type, worker_name, expires_at, check_type, check_source, plan_id, working_directory, env_variables, attempts)
		VALUES ($1, $2, $3, $4, $5, $6,  $7, NOW() + $8::INTERVAL, $9, $10, $11, $12, $13, $14)
		RETURNING `+containerIdentifierColumns,
		container.Handle,
		resourceID,
		container.StepName,
		pipelineID,
		buildID,
		container.Type.String(),
		workerName,
		interval,
		container.CheckType,
		checkSource,
		string(container.PlanID),
		container.WorkingDirectory,
		envVariables,
		attempts,
	))

	if err != nil {
		return newContainer, err
	}

	err = tx.Commit()
	if err != nil {
		return newContainer, err
	}

	return newContainer, nil
}

func (db *SQLDB) UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error {
	tx, err := db.conn.Begin()

	if err != nil {
		return err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE containers SET expires_at = NOW() + $2::INTERVAL
		WHERE handle = $1
		`,
		handle,
		interval,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) ReapContainer(handle string) error {
	rows, err := db.conn.Exec(`
		DELETE FROM containers WHERE handle = $1
	`, handle)
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	// just to be explicit: reaping 0 containers is fine;
	// it may have already expired
	if affected == 0 {
		return nil
	}

	return nil
}

func (db *SQLDB) DeleteContainer(handle string) error {
	_, err := db.conn.Exec(`
		DELETE FROM containers
		WHERE handle = $1
	`, handle)
	return err
}

func scanContainerMetadata(row scannable) (Container, error) {
	var (
		infoType         string
		checkSourceBlob  []byte
		envVariablesBlob []byte
		resourceName     sql.NullString
		pipelineID       sql.NullInt64
		pipelineName     sql.NullString
		jobName          sql.NullString
		buildID          sql.NullInt64
		buildName        sql.NullString
		attempts         sql.NullString
	)
	container := Container{}

	err := row.Scan(
		&container.Handle,
		&container.ExpiresAt,
		&buildID,
		&buildName,
		&resourceName,
		&container.ContainerMetadata.WorkerName,
		&pipelineID,
		&pipelineName,
		&jobName,
		&container.StepName,
		&infoType,
		&container.WorkingDirectory,
		&container.CheckType,
		&checkSourceBlob,
		&envVariablesBlob,
		&attempts,
	)
	if err != nil {
		return Container{}, err
	}

	if buildID.Valid {
		container.ContainerMetadata.BuildID = int(buildID.Int64)
	}

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
		return Container{}, err
	}

	err = json.Unmarshal(checkSourceBlob, &container.CheckSource)
	if err != nil {
		return Container{}, err
	}

	err = json.Unmarshal(envVariablesBlob, &container.EnvironmentVariables)
	if err != nil {
		return Container{}, err
	}

	if attempts.Valid {
		for _, item := range strings.Split(attempts.String, ",") {
			attemptInt, err := strconv.Atoi(item)
			if err != nil {
				return Container{}, err
			}
			container.Attempts = append(container.Attempts, attemptInt)
		}
	}

	return container, nil
}

func scanContainerIdentifier(row scannable) (Container, error) {
	var planID sql.NullString
	var resourceID sql.NullInt64
	var buildID sql.NullInt64
	container := Container{}

	err := row.Scan(
		&container.Handle,
		&container.ExpiresAt,
		&container.ContainerIdentifier.WorkerName,
		&resourceID,
		&buildID,
		&planID,
	)
	if err != nil {
		return Container{}, err
	}

	if resourceID.Valid {
		container.ResourceID = int(resourceID.Int64)
	}

	if buildID.Valid {
		container.ContainerIdentifier.BuildID = int(buildID.Int64)
	}

	container.PlanID = atc.PlanID(planID.String)

	return container, nil
}

func deleteExpired(db *SQLDB) error {
	_, err := db.conn.Exec(`
		DELETE FROM containers
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	return err
}
