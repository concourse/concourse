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

func (db *SQLDB) FindJobContainersFromUnsuccessfulBuilds() ([]SavedContainer, error) {
	rows, err := db.conn.Query(
		`SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins + `
		WHERE (b.status = 'failed' OR b.status = 'errored')
		AND b.job_id is not null`)

	if err != nil {
		if err == sql.ErrNoRows {
			return []SavedContainer{}, nil
		}
		return nil, err
	}

	return scanRows(rows)
}

func (db *SQLDB) FindContainerByIdentifier(id ContainerIdentifier) (SavedContainer, bool, error) {
	conditions := []string{}
	params := []interface{}{}
	extraJoins := ""

	addParam := func(column string, param interface{}) {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(params)+1))
		params = append(params, param)
	}

	if id.ImageResourceSource != nil && id.ImageResourceType != "" {
		marshaled, err := json.Marshal(id.ImageResourceSource)
		if err != nil {
			return SavedContainer{}, false, err
		}

		addParam("image_resource_source", string(marshaled))
		addParam("image_resource_type", id.ImageResourceType)
	} else {
		conditions = append(conditions, []string{
			"image_resource_source IS NULL",
			"image_resource_type IS NULL",
		}...)
	}

	addParam("build_id", id.BuildID)
	addParam("plan_id", string(id.PlanID))
	addParam("stage", string(id.Stage))

	selectQuery := `
		SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins + `
		LEFT JOIN workers w ON c.worker_name = w.name ` + extraJoins + `
		WHERE w.state = 'running'
		AND c.state = 'created'
		AND ` + strings.Join(conditions, " AND ")

	rows, err := db.conn.Query(selectQuery, params...)
	if err != nil {
		return SavedContainer{}, false, err
	}

	var containers []SavedContainer
	for rows.Next() {
		container, err := scanContainer(rows)
		if err != nil {
			return SavedContainer{}, false, nil
		}
		containers = append(containers, container)
	}

	switch len(containers) {
	case 0:
		return SavedContainer{}, false, nil

	case 1:
		return containers[0], true, nil

	default:
		return SavedContainer{}, false, ErrMultipleContainersFound
	}
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
			attempts
		) = (
			`+maxLifetimeValue+`,
			$2,
			$3,
			$4,
			$5
		)
		WHERE handle = $1
		RETURNING id`,
		container.Handle,
		imageResourceType,
		imageResourceSource,
		container.User,
		attempts,
	).Scan(&id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) CreateContainerToBeRemoved(container Container, maxLifetime time.Duration, volumeHandles []string) (SavedContainer, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return SavedContainer{}, err
	}

	defer tx.Rollback()

	envVariables, err := json.Marshal(container.EnvironmentVariables)
	if err != nil {
		return SavedContainer{}, err
	}

	user := container.User

	if container.PipelineName != "" && container.PipelineID == 0 {
		// containers that belong to some pipeline must be identified by pipeline ID not name
		return SavedContainer{}, errors.New("container metadata must include pipeline ID")
	}
	var pipelineID sql.NullInt64
	if container.PipelineID != 0 {
		pipelineID.Int64 = int64(container.PipelineID)
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

	var attempts sql.NullString
	if len(container.Attempts) > 0 {
		attemptsBlob, err := json.Marshal(container.Attempts)
		if err != nil {
			return SavedContainer{}, err
		}
		attempts.Valid = true
		attempts.String = string(attemptsBlob)
	}

	var imageResourceSource sql.NullString
	if container.ImageResourceSource != nil {
		marshaled, err := json.Marshal(container.ImageResourceSource)
		if err != nil {
			return SavedContainer{}, err
		}

		imageResourceSource.String = string(marshaled)
		imageResourceSource.Valid = true
	}

	var imageResourceType sql.NullString
	if container.ImageResourceType != "" {
		imageResourceType.String = container.ImageResourceType
		imageResourceType.Valid = true
	}

	maxLifetimeValue := "NULL"
	if maxLifetime > 0 {
		maxLifetimeValue = fmt.Sprintf(`NOW() + '%d second'::INTERVAL`, int(maxLifetime.Seconds()))
	}

	var id int
	err = tx.QueryRow(`
		INSERT INTO containers (handle, state, resource_id, step_name, pipeline_id, build_id, type, worker_name,
			best_if_used_by, plan_id, working_directory,
			env_variables, attempts, stage, image_resource_type, image_resource_source,
			process_user, team_id)
		VALUES ($1, 'created', $2, $3, $4, $5, $6, $7, `+maxLifetimeValue+`, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id`,
		container.Handle,
		resourceID,
		container.StepName,
		pipelineID,
		buildID,
		container.Type.String(),
		container.WorkerName,
		string(container.PlanID),
		container.WorkingDirectory,
		envVariables,
		attempts,
		string(container.Stage),
		imageResourceType,
		imageResourceSource,
		user,
		container.TeamID,
	).Scan(&id)
	if err != nil {
		return SavedContainer{}, err
	}

	newContainer, err := scanContainer(tx.QueryRow(`
		SELECT `+containerColumns+`
	  FROM containers c `+containerJoins+`
		WHERE c.id = $1
	`, id))
	if err != nil {
		return SavedContainer{}, err
	}

	for _, volumeHandle := range volumeHandles {
		// transition to initialized
		_, err = tx.Exec(`
			UPDATE volumes
			SET container_id = $1
			WHERE handle = $2
		`, id, volumeHandle)
		if err != nil {
			return SavedContainer{}, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return SavedContainer{}, err
	}

	return newContainer, nil
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
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		DELETE FROM containers WHERE handle = $1
	`, handle)
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
