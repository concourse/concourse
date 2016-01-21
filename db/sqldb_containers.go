package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/concourse/atc"
)

const containerColumns = "worker_name, resource_id, check_type, check_source, build_id, plan_id, stage, handle, b.name as build_name, r.name as resource_name, p.id as pipeline_id, p.name as pipeline_name, j.name as job_name, step_name, type, working_directory, env_variables, attempts"
const containerJoins = `
		LEFT JOIN pipelines p
		  ON p.id = c.pipeline_id
		LEFT JOIN resources r
			ON r.id = c.resource_id
		LEFT JOIN builds b
		  ON b.id = c.build_id
		LEFT JOIN jobs j
		  ON j.id = b.job_id`

func (db *SQLDB) FindContainersByDescriptors(id Container) ([]Container, error) {
	err := deleteExpired(db)
	if err != nil {
		return nil, err
	}

	var whereCriteria []string
	var params []interface{}

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

	if len(id.Attempts) > 0 {
		whereCriteria = append(whereCriteria, fmt.Sprintf("attempts = $%d", len(params)+1))
		params = append(params, AttemptsStringFromSlice(id.Attempts))
	}

	var rows *sql.Rows
	selectQuery := `
		SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins

	if len(whereCriteria) > 0 {
		selectQuery += fmt.Sprintf(" WHERE %s", strings.Join(whereCriteria, " AND "))
	}

	rows, err = db.conn.Query(selectQuery, params...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	infos := []Container{}
	for rows.Next() {
		info, err := scanContainer(rows)

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
			SELECT ` + containerColumns + `
	    FROM containers c ` + containerJoins + `
		  WHERE resource_id = $1
	  	`
		rows, err = db.conn.Query(selectQuery, id.ResourceID)
	} else if id.BuildID != 0 && id.PlanID != "" {
		selectQuery = `
			SELECT ` + containerColumns + `
	    FROM containers c ` + containerJoins + `
		  WHERE build_id = $1 AND plan_id = $2
			AND stage = $3
	  	`
		rows, err = db.conn.Query(selectQuery, id.BuildID, string(id.PlanID), string(id.Stage))
	} else {
		return Container{}, false, errors.New("insufficient container identifiers")
	}

	if err != nil {
		return Container{}, false, err
	}

	for rows.Next() {
		container, err := scanContainer(rows)
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
	err := deleteExpired(db)
	if err != nil {
		return Container{}, false, err
	}

	container, err := scanContainer(db.conn.QueryRow(`
		SELECT `+containerColumns+`
	  FROM containers c `+containerJoins+`
		WHERE c.handle = $1
	`, handle))

	if err != nil {
		if err == sql.ErrNoRows {
			return Container{}, false, nil
		}
		return Container{}, false, err
	}

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
	if container.BuildID != 0 {
		buildID.Int64 = int64(container.BuildID)
		buildID.Valid = true
	}

	workerName := container.WorkerName
	if workerName == "" {
		workerName = container.WorkerName
	}

	var attempts sql.NullString
	attempts.String = AttemptsStringFromSlice(container.Attempts)
	if attempts.String != "" {
		attempts.Valid = true
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO containers (handle, resource_id, step_name, pipeline_id, build_id, type, worker_name, expires_at, check_type, check_source, plan_id, working_directory, env_variables, attempts, stage)
		VALUES ($1, $2, $3, $4, $5, $6,  $7, NOW() + $8::INTERVAL, $9, $10, $11, $12, $13, $14, $15)`,
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
		string(container.Stage),
	)
	if err != nil {
		return Container{}, err
	}

	newContainer, err := scanContainer(tx.QueryRow(`
		SELECT `+containerColumns+`
	  FROM containers c `+containerJoins+`
		WHERE c.handle = $1
	`, container.Handle))
	if err != nil {
		return Container{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Container{}, err
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

func scanContainer(row scannable) (Container, error) {
	var (
		resourceID       sql.NullInt64
		checkSourceBlob  []byte
		buildID          sql.NullInt64
		planID           sql.NullString
		stage            string
		buildName        sql.NullString
		resourceName     sql.NullString
		pipelineID       sql.NullInt64
		pipelineName     sql.NullString
		jobName          sql.NullString
		infoType         string
		envVariablesBlob []byte
		attempts         sql.NullString
	)
	container := Container{}

	err := row.Scan(
		&container.WorkerName,
		&resourceID,
		&container.CheckType,
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
		attemptsSlice, err := AttemptsSliceFromString(attempts.String)
		if err != nil {
			return Container{}, err
		}
		container.Attempts = attemptsSlice
	}

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
