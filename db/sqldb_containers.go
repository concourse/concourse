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

const containerColumns = "worker_name, resource_id, check_type, check_source, build_id, plan_id, stage, handle, b.name as build_name, r.name as resource_name, p.id as pipeline_id, p.name as pipeline_name, j.name as job_name, step_name, type, working_directory, env_variables, attempts, process_user, ttl, EXTRACT(epoch FROM expires_at - NOW())"
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

func (db *SQLDB) FindContainersByDescriptors(id Container) ([]SavedContainer, error) {
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

	if id.BuildName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("b.name = $%d", len(params)+1))
		params = append(params, id.BuildName)
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
		attemptsBlob, err := json.Marshal(id.Attempts)
		if err != nil {
			return nil, err
		}
		whereCriteria = append(whereCriteria, fmt.Sprintf("attempts = $%d", len(params)+1))
		params = append(params, attemptsBlob)
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

	infos := []SavedContainer{}
	for rows.Next() {
		info, err := scanContainer(rows)

		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func (db *SQLDB) FindContainersFromSuccessfulBuildsWithInfiniteTTL() ([]SavedContainer, error) {
	rows, err := db.conn.Query(
		`SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins + `
		WHERE b.status = 'succeeded'
		AND c.ttl = '0'`)

	if err != nil {
		return nil, err
	}

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

func (db *SQLDB) FindContainersFromUnsuccessfulBuildsWithInfiniteTTL() ([]SavedContainer, error) {
	rows, err := db.conn.Query(
		`SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins + `
		WHERE (b.status = 'failed' OR b.status = 'aborted' OR b.status = 'errored')
		AND c.ttl = '0'`)

	if err != nil {
		return nil, err
	}

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

func (db *SQLDB) FindContainerByIdentifier(id ContainerIdentifier) (SavedContainer, bool, error) {
	err := deleteExpired(db)
	if err != nil {
		return SavedContainer{}, false, err
	}

	conditions := []string{}
	params := []interface{}{}

	addParam := func(column string, param interface{}) {
		conditions = append(conditions, fmt.Sprintf("%s = $%d", column, len(params)+1))
		params = append(params, param)
	}

	switch {
	case isValidCheckID(id):
		checkSourceBlob, err := json.Marshal(id.CheckSource)
		if err != nil {
			return SavedContainer{}, false, err
		}

		if id.ResourceID > 0 {
			addParam("resource_id", id.ResourceID)
		}
		addParam("check_type", id.CheckType)
		addParam("check_source", checkSourceBlob)
		addParam("stage", string(id.Stage))
		conditions = append(conditions, "(best_if_used_by IS NULL OR best_if_used_by > NOW())")
	case isValidStepID(id):
		addParam("build_id", id.BuildID)
		addParam("plan_id", string(id.PlanID))
		addParam("stage", string(id.Stage))
	default:
		return SavedContainer{}, false, ErrInvalidIdentifier
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

	selectQuery := `
		SELECT ` + containerColumns + `
		FROM containers c ` + containerJoins + `
		WHERE ` + strings.Join(conditions, " AND ")

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
	err := deleteExpired(db)
	if err != nil {
		return SavedContainer{}, false, err
	}

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

func (db *SQLDB) CreateContainer(
	container Container,
	ttl time.Duration,
	maxLifetime time.Duration,
) (SavedContainer, error) {
	if !(isValidCheckID(container.ContainerIdentifier) || isValidStepID(container.ContainerIdentifier)) {
		return SavedContainer{}, ErrInvalidIdentifier
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return SavedContainer{}, err
	}

	defer tx.Rollback()

	checkSource, err := json.Marshal(container.CheckSource)
	if err != nil {
		return SavedContainer{}, err
	}

	envVariables, err := json.Marshal(container.EnvironmentVariables)
	if err != nil {
		return SavedContainer{}, err
	}

	user := container.User

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

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

	var resourceTypeVersion string
	if container.ResourceTypeVersion != nil {
		resourceTypeVersionBytes, err := json.Marshal(container.ResourceTypeVersion)
		if err != nil {
			return SavedContainer{}, err
		}
		resourceTypeVersion = string(resourceTypeVersionBytes)
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

	_, err = tx.Exec(`
		INSERT INTO containers (handle, resource_id, step_name, pipeline_id, build_id, type, worker_name, expires_at, ttl, best_if_used_by, check_type, check_source, plan_id, working_directory, env_variables, attempts, stage, image_resource_type, image_resource_source, process_user, resource_type_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW() + $8::INTERVAL, $9,`+maxLifetimeValue+`, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		container.Handle,
		resourceID,
		container.StepName,
		pipelineID,
		buildID,
		container.Type.String(),
		workerName,
		interval,
		ttl,
		container.CheckType,
		checkSource,
		string(container.PlanID),
		container.WorkingDirectory,
		envVariables,
		attempts,
		string(container.Stage),
		imageResourceType,
		imageResourceSource,
		user,
		resourceTypeVersion,
	)
	if err != nil {
		return SavedContainer{}, err
	}

	newContainer, err := scanContainer(tx.QueryRow(`
		SELECT `+containerColumns+`
	  FROM containers c `+containerJoins+`
		WHERE c.handle = $1
	`, container.Handle))
	if err != nil {
		return SavedContainer{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedContainer{}, err
	}

	return newContainer, nil
}

func (db *SQLDB) UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	if ttl == 0 {
		_, err = tx.Exec(`
			UPDATE containers SET expires_at = NULL, ttl = 0
			WHERE handle = $1
			`, handle)

		if err != nil {
			return err
		}
	} else {
		interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

		_, err = tx.Exec(`
				UPDATE containers SET expires_at = NOW() + $2::INTERVAL, ttl = $3
				WHERE handle = $1
				`,
			handle,
			interval,
			ttl,
		)

		if err != nil {
			return err
		}
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

func isValidCheckID(id ContainerIdentifier) bool {
	if !((id.ResourceID > 0 || id.ResourceTypeVersion != nil) &&
		id.CheckType != "" &&
		id.CheckSource != nil &&
		id.BuildID == 0 &&
		id.PlanID == "") {
		return false
	}

	switch id.Stage {
	case ContainerStageCheck, ContainerStageGet:
		return id.ImageResourceType != "" && id.ImageResourceSource != nil
	case ContainerStageRun:
		return id.ImageResourceType == "" && id.ImageResourceSource == nil
	default:
		return false
	}
}

func isValidStepID(id ContainerIdentifier) bool {
	if !(id.ResourceID == 0 &&
		id.CheckType == "" &&
		id.CheckSource == nil &&
		id.BuildID > 0 &&
		id.PlanID != "") {
		return false
	}

	switch id.Stage {
	case ContainerStageCheck, ContainerStageGet:
		return id.ImageResourceType != "" && id.ImageResourceSource != nil
	case ContainerStageRun:
		return id.ImageResourceType == "" && id.ImageResourceSource == nil
	default:
		return false
	}
}

func scanContainer(row scannable) (SavedContainer, error) {
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
		ttlInSeconds     *float64
	)
	container := SavedContainer{}

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
		&container.User,
		&container.TTL,
		&ttlInSeconds,
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

	err = json.Unmarshal(checkSourceBlob, &container.CheckSource)
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

	if ttlInSeconds != nil {
		container.ExpiresIn = time.Duration(*ttlInSeconds) * time.Second
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
