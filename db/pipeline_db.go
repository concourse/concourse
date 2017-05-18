package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . PipelineDB

type PipelineDB interface {
	Pipeline() SavedPipeline
	GetPipelineName() string
	GetPipelineID() int
	ScopedName(string) string
	TeamID() int
	Config() atc.Config
	ConfigVersion() ConfigVersion

	Reload() (bool, error)

	Pause() error
	Unpause() error
	IsPaused() (bool, error)
	IsPublic() bool

	AcquireSchedulingLock(lager.Logger, time.Duration) (lock.Lock, bool, error)

	GetResource(resourceName string) (SavedResource, bool, error)
	GetResourceType(resourceTypeName string) (SavedResourceType, bool, error)

	SaveResourceTypeVersion(atc.ResourceType, atc.Version) error
	EnableVersionedResource(versionedResourceID int) error
	DisableVersionedResource(versionedResourceID int) error
	SetResourceCheckError(resource SavedResource, err error) error

	GetJob(job string) (SavedJob, bool, error)

	GetVersionedResourceByVersion(atcVersion atc.Version, resourceName string) (SavedVersionedResource, bool, error)
	SaveIndependentInputMapping(inputMapping algorithm.InputMapping, jobName string) error
	SaveNextInputMapping(inputMapping algorithm.InputMapping, jobName string) error

	// possibly move to job.go
	PauseJob(job string) error
	UnpauseJob(job string) error
	GetNextBuildInputs(jobName string) ([]BuildInput, bool, error)
	DeleteNextInputMapping(jobName string) error
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error)
	GetJobFinishedAndNextBuild(job string) (Build, Build, error)
	GetJobBuilds(job string, page Page) ([]Build, Pagination, error)
	GetAllJobBuilds(job string) ([]Build, error)
	GetJobBuild(job string, build string) (Build, bool, error)
	CreateJobBuild(job string) (Build, error)
	SetMaxInFlightReached(job string, reached bool) error
	UpdateFirstLoggedBuildID(job string, newFirstLoggedBuildID int) error

	UpdateBuildToScheduled(buildID int) (bool, error)
	GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error)
	GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error)

	Expose() error
	Hide() error
}

type pipelineDB struct {
	conn Conn
	bus  *notificationsBus

	SavedPipeline

	versionsDB *algorithm.VersionsDB

	lockFactory  lock.LockFactory
	buildFactory *buildFactory
}

type ResourceNotFoundError struct {
	Name string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.Name)
}

type ResourceTypeNotFoundError struct {
	Name string
}

func (e ResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("resource type '%s' not found", e.Name)
}

type FirstLoggedBuildIDDecreasedError struct {
	Job   string
	OldID int
	NewID int
}

func (e FirstLoggedBuildIDDecreasedError) Error() string {
	return fmt.Sprintf("first logged build id for job '%s' decreased from %d to %d", e.Job, e.OldID, e.NewID)
}

func (pdb *pipelineDB) Pipeline() SavedPipeline {
	return pdb.SavedPipeline
}

func (pdb *pipelineDB) GetPipelineName() string {
	return pdb.Name
}

func (pdb *pipelineDB) GetPipelineID() int {
	return pdb.ID
}

func (pdb *pipelineDB) ScopedName(name string) string {
	return pdb.Name + ":" + name
}

func (pdb *pipelineDB) TeamID() int {
	return pdb.SavedPipeline.TeamID
}

func (pdb *pipelineDB) Config() atc.Config {
	return pdb.SavedPipeline.Config
}

func (pdb *pipelineDB) ConfigVersion() ConfigVersion {
	return pdb.SavedPipeline.Version
}

func (pdb *pipelineDB) IsPublic() bool {
	return pdb.Public
}

func (pdb *pipelineDB) Unpause() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET paused = false
		WHERE id = $1
	`, pdb.ID)
	return err
}

func (pdb *pipelineDB) Pause() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET paused = true
		WHERE id = $1
	`, pdb.ID)
	return err
}

func (pdb *pipelineDB) Reload() (bool, error) {
	row := pdb.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, pdb.ID)

	savedPipeline, err := scanPipeline(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	pdb.SavedPipeline = savedPipeline

	return true, nil
}

func (pdb *pipelineDB) GetResource(resourceName string) (SavedResource, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedResource{}, false, err
	}

	defer tx.Rollback()

	resource, found, err := pdb.getResource(tx, resourceName)
	if err != nil {
		return SavedResource{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedResource{}, false, err
	}

	return resource, found, nil
}

func (pdb *pipelineDB) AcquireSchedulingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE pipelines
		SET last_scheduled = now()
		WHERE id = $1
			AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
	`, pdb.ID, interval.Seconds())
	if err != nil {
		return nil, false, err
	}

	if !updated {
		return nil, false, nil
	}

	lock := pdb.lockFactory.NewLock(
		logger.Session("lock", lager.Data{
			"pipeline": pdb.Name,
		}),
		lock.NewPipelineSchedulingLockLockID(pdb.ID),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	err = tx.Commit()
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	return lock, true, nil
}

func (pdb *pipelineDB) getResource(tx Tx, name string) (SavedResource, bool, error) {
	return pdb.scanResource(tx.QueryRow(`
			SELECT id, name, config, check_error, paused
			FROM resources
			WHERE name = $1
				AND pipeline_id = $2
				AND active = true
		`, name, pdb.ID))
}

func (pdb *pipelineDB) scanResource(row scannable) (SavedResource, bool, error) {
	var checkErr sql.NullString
	var resource SavedResource
	var configBlob []byte

	err := row.Scan(&resource.ID, &resource.Name, &configBlob, &checkErr, &resource.Paused)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedResource{}, false, nil
		}

		return SavedResource{}, false, err
	}

	resource.PipelineName = pdb.GetPipelineName()

	var config atc.ResourceConfig
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedResource{}, false, err
	}
	resource.Config = config

	if checkErr.Valid {
		resource.CheckError = errors.New(checkErr.String)
	}

	return resource, true, nil
}

func (pdb *pipelineDB) GetResourceType(name string) (SavedResourceType, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedResourceType{}, false, err
	}

	defer tx.Rollback()

	resourceType, found, err := pdb.getResourceType(tx, name)
	if err != nil {
		return SavedResourceType{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedResourceType{}, false, err
	}

	return resourceType, found, nil
}

func (pdb *pipelineDB) getResourceType(tx Tx, name string) (SavedResourceType, bool, error) {
	var savedResourceType SavedResourceType
	var versionJSON []byte
	var configBlob []byte
	err := tx.QueryRow(`
			SELECT id, name, type, version, config
			FROM resource_types
			WHERE name = $1
				AND pipeline_id = $2
				AND active = true
		`, name, pdb.ID).Scan(&savedResourceType.ID, &savedResourceType.Name, &savedResourceType.Type, &versionJSON, &configBlob)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedResourceType{}, false, nil
		}
		return SavedResourceType{}, false, err
	}

	var config atc.ResourceType
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedResourceType{}, false, err
	}
	savedResourceType.Config = config

	if versionJSON != nil {
		err := json.Unmarshal(versionJSON, &savedResourceType.Version)
		if err != nil {
			return SavedResourceType{}, false, err
		}
	}

	return savedResourceType, true, nil
}

func (pdb *pipelineDB) SaveResourceTypeVersion(resourceType atc.ResourceType, version atc.Version) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	versionJSON, err := json.Marshal(version)
	if err != nil {
		return err
	}

	_, found, err := pdb.getResourceType(tx, resourceType.Name)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("failed-to-find-resource-type")
	}

	_, err = tx.Exec(`
		UPDATE resource_types
		SET version = $1
		WHERE name = $2
		AND type = $3
		AND pipeline_id = $4
		AND active = true
	`, string(versionJSON), resourceType.Name, resourceType.Type, pdb.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (pdb *pipelineDB) DisableVersionedResource(versionedResourceID int) error {
	return pdb.toggleVersionedResource(versionedResourceID, false)
}

func (pdb *pipelineDB) EnableVersionedResource(versionedResourceID int) error {
	return pdb.toggleVersionedResource(versionedResourceID, true)
}

func (pdb *pipelineDB) toggleVersionedResource(versionedResourceID int, enable bool) error {
	rows, err := pdb.conn.Exec(`
		UPDATE versioned_resources
		SET enabled = $1, modified_time = now()
		WHERE id = $2
	`, enable, versionedResourceID)
	if err != nil {
		return err
	}

	rowsAffected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (pdb *pipelineDB) SetResourceCheckError(resource SavedResource, cause error) error {
	var err error

	if cause == nil {
		_, err = pdb.conn.Exec(`
			UPDATE resources
			SET check_error = NULL
			WHERE id = $1
			`, resource.ID)
	} else {
		_, err = pdb.conn.Exec(`
			UPDATE resources
			SET check_error = $2
			WHERE id = $1
		`, resource.ID, cause.Error())
	}

	return err
}

func (pdb *pipelineDB) incrementCheckOrderWhenNewerVersion(tx Tx, resourceID int, resourceType string, version string) error {
	_, err := tx.Exec(`
		WITH max_checkorder AS (
			SELECT max(check_order) co
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
		)

		UPDATE versioned_resources
		SET check_order = mc.co + 1
		FROM max_checkorder mc
		WHERE resource_id = $1
		AND type = $2
		AND version = $3
		AND check_order <= mc.co;`, resourceID, resourceType, version)
	if err != nil {
		return err
	}

	return nil
}

func (pdb *pipelineDB) GetJob(jobName string) (SavedJob, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedJob{}, false, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedJob{}, false, nil
		}
		return SavedJob{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedJob{}, false, err
	}

	return dbJob, true, nil
}

func (pdb *pipelineDB) GetJobBuild(job string, name string) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return nil, false, err
	}

	build, found, err := pdb.buildFactory.ScanBuild(tx.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		WHERE b.job_id = $1
		AND b.name = $2
	`, dbJob.ID, name))
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return build, found, nil
}

func (pdb *pipelineDB) CreateJobBuild(jobName string) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	buildName, jobID, err := getNewBuildNameForJob(tx, jobName, pdb.ID)
	if err != nil {
		return nil, err
	}

	// We had to resort to sub-selects here because you can't paramaterize a
	// RETURNING statement in lib/pq... sorry
	build, _, err := pdb.buildFactory.ScanBuild(tx.QueryRow(`
		INSERT INTO builds (name, job_id, team_id, status, manually_triggered)
		VALUES ($1, $2, $3, 'pending', TRUE)
		RETURNING `+buildColumns+`,
			(SELECT name FROM jobs WHERE id = $2),
			(SELECT id FROM pipelines WHERE id = $4),
			(SELECT name FROM pipelines WHERE id = $4),
			(SELECT name FROM teams WHERE id = $3)
	`, buildName, jobID, pdb.SavedPipeline.TeamID, pdb.ID))
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

func getNewBuildNameForJob(tx Tx, jobName string, pipelineID int) (string, int, error) {
	var buildName string
	var jobID int
	err := tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1 AND pipeline_id = $2
		RETURNING build_number_seq, id
	`, jobName, pipelineID).Scan(&buildName, &jobID)
	return buildName, jobID, err
}

func createBuildEventSeq(tx Tx, buildID int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(buildID)))
	return err
}

func (pdb *pipelineDB) GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		INNER JOIN build_inputs bi ON bi.build_id = b.id
		WHERE bi.versioned_resource_id = $1
	`, versionedResourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	builds := []Build{}
	for rows.Next() {
		build, _, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}
		builds = append(builds, build)
	}

	return builds, err
}

func (pdb *pipelineDB) GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		INNER JOIN build_outputs bo ON bo.build_id = b.id
		WHERE bo.versioned_resource_id = $1
	`, versionedResourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	builds := []Build{}
	for rows.Next() {
		build, _, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, err
}

func (pdb *pipelineDB) updateSerialGroupsForJob(jobName string, serialGroups []string) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM jobs_serial_groups
		WHERE job_id = $1
	`, dbJob.ID)
	if err != nil {
		return err
	}

	for _, serialGroup := range serialGroups {
		_, err = tx.Exec(`
			INSERT INTO jobs_serial_groups (job_id, serial_group)
			VALUES ($1, $2)
		`, dbJob.ID, serialGroup)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	args := []interface{}{pdb.ID}
	refs := make([]string, len(serialGroups))

	for i, serialGroup := range serialGroups {
		args = append(args, serialGroup)
		refs[i] = fmt.Sprintf("$%d", i+2)
	}

	return pdb.buildFactory.ScanBuild(pdb.conn.QueryRow(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE b.status = 'pending'
			AND j.inputs_determined = true
			AND j.pipeline_id = $1
		ORDER BY b.id ASC
		LIMIT 1
	`, args...))
}

func (pdb *pipelineDB) GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	args := []interface{}{pdb.ID}
	refs := make([]string, len(serialGroups))

	for i, serialGroup := range serialGroups {
		args = append(args, serialGroup)
		refs[i] = fmt.Sprintf("$%d", i+2)
	}

	rows, err := pdb.conn.Query(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE (
				b.status = 'started'
				OR
				(b.scheduled = true AND b.status = 'pending')
			)
			AND j.pipeline_id = $1
	`, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, _, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) IsPaused() (bool, error) {
	var paused bool

	err := pdb.conn.QueryRow(`
		SELECT paused
		FROM pipelines
		WHERE id = $1
	`, pdb.ID).Scan(&paused)

	if err != nil {
		return false, err
	}

	return paused, nil
}

func (pdb *pipelineDB) UpdateBuildToScheduled(buildID int) (bool, error) {
	result, err := pdb.conn.Exec(`
			UPDATE builds
			SET scheduled = true
			WHERE id = $1
	`, buildID)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (pdb *pipelineDB) GetVersionedResourceByVersion(atcVersion atc.Version, resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	versionJSON, err := json.Marshal(atcVersion)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource:   resourceName,
			PipelineID: pdb.GetPipelineID(),
		},
	}

	err = pdb.conn.QueryRow(`
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, v.check_order
		FROM versioned_resources v
		JOIN resources r ON r.id = v.resource_id
		WHERE v.version = $1
			AND r.name = $2
			AND r.pipeline_id = $3
			AND enabled = true
	`, string(versionJSON), resourceName, pdb.ID).Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.CheckOrder)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedVersionedResource{}, false, nil
		}

		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(versionBytes), &svr.Version)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &svr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	return svr, true, nil
}

func (pdb *pipelineDB) SaveIndependentInputMapping(inputMapping algorithm.InputMapping, jobName string) error {
	return pdb.saveJobInputMapping("independent_build_inputs", inputMapping, jobName)
}

func (pdb *pipelineDB) SaveNextInputMapping(inputMapping algorithm.InputMapping, jobName string) error {
	return pdb.saveJobInputMapping("next_build_inputs", inputMapping, jobName)
}

func (pdb *pipelineDB) GetNextBuildInputs(jobName string) ([]BuildInput, bool, error) {
	var found bool
	err := pdb.conn.QueryRow(`
			SELECT inputs_determined FROM jobs WHERE name = $1 AND pipeline_id = $2
		`, jobName, pdb.ID).Scan(&found)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// there is a possible race condition where found is true at first but the
	// inputs are deleted by the time we get here
	buildInputs, err := pdb.getJobBuildInputs("next_build_inputs", jobName)
	return buildInputs, true, err
}

func (pdb *pipelineDB) DeleteNextInputMapping(jobName string) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var jobID int
	err = tx.QueryRow(`
		UPDATE jobs
		SET inputs_determined = false
		WHERE name = $1 AND pipeline_id = $2
		RETURNING id
		`, jobName, pdb.ID).Scan(&jobID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM next_build_inputs WHERE job_id = $1
		`, jobID)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}

func (pdb *pipelineDB) saveJobInputMapping(table string, inputMapping algorithm.InputMapping, jobName string) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var jobID int
	switch table {
	case "independent_build_inputs":
		err = tx.QueryRow(`
			SELECT id FROM jobs WHERE name = $1 AND pipeline_id = $2
			`, jobName, pdb.ID).Scan(&jobID)
	case "next_build_inputs":
		err = tx.QueryRow(`
			UPDATE jobs
			SET inputs_determined = true
			WHERE name = $1 AND pipeline_id = $2
			RETURNING id
			`, jobName, pdb.ID).Scan(&jobID)
	default:
		panic("unknown table " + table)
	}
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
    SELECT input_name, version_id, first_occurrence
		FROM `+table+`
		WHERE job_id = $1
  `, jobID)
	if err != nil {
		return err
	}

	oldInputMapping := algorithm.InputMapping{}
	for rows.Next() {
		var inputName string
		var inputVersion algorithm.InputVersion
		err := rows.Scan(&inputName, &inputVersion.VersionID, &inputVersion.FirstOccurrence)
		if err != nil {
			return err
		}

		oldInputMapping[inputName] = inputVersion
	}

	for inputName, oldInputVersion := range oldInputMapping {
		inputVersion, found := inputMapping[inputName]
		if !found || inputVersion != oldInputVersion {
			_, err = tx.Exec(`
				DELETE FROM `+table+` WHERE job_id = $1 AND input_name = $2
			`, jobID, inputName)
			if err != nil {
				return err
			}
		}
	}

	for inputName, inputVersion := range inputMapping {
		oldInputVersion, found := oldInputMapping[inputName]
		if !found || inputVersion != oldInputVersion {
			_, err := tx.Exec(`
				INSERT INTO `+table+` (job_id, input_name, version_id, first_occurrence)
				VALUES ($1, $2, $3, $4)
			`, jobID, inputName, inputVersion.VersionID, inputVersion.FirstOccurrence)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) getJobBuildInputs(table string, jobName string) ([]BuildInput, error) {
	rows, err := pdb.conn.Query(`
		SELECT i.input_name, i.first_occurrence, r.name, v.type, v.version, v.metadata
		FROM `+table+` i
		JOIN jobs j ON i.job_id = j.id
		JOIN versioned_resources v ON v.id = i.version_id
		JOIN resources r ON r.id = v.resource_id
		WHERE j.name = $1
		AND j.pipeline_id = $2
		`, jobName, pdb.ID)

	if err != nil {
		return nil, err
	}

	buildInputs := []BuildInput{}
	for rows.Next() {
		var (
			inputName       string
			firstOccurrence bool
			resourceName    string
			resourceType    string
			versionBlob     string
			metadataBlob    string
			version         Version
			metadata        []MetadataField
		)

		err := rows.Scan(&inputName, &firstOccurrence, &resourceName, &resourceType, &versionBlob, &metadataBlob)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(metadataBlob), &metadata)
		if err != nil {
			return nil, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name: inputName,
			VersionedResource: VersionedResource{
				Resource:   resourceName,
				Type:       resourceType,
				Version:    version,
				Metadata:   metadata,
				PipelineID: pdb.ID,
			},
			FirstOccurrence: firstOccurrence,
		})
	}
	return buildInputs, nil
}

func (pdb *pipelineDB) PauseJob(job string) error {
	return pdb.updatePausedJob(job, true)
}

func (pdb *pipelineDB) UnpauseJob(job string) error {
	return pdb.updatePausedJob(job, false)
}

func (pdb *pipelineDB) SetMaxInFlightReached(jobName string, reached bool) error {
	result, err := pdb.conn.Exec(`
		UPDATE jobs
		SET max_in_flight_reached = $1
		WHERE name = $2 AND pipeline_id = $3
	`, reached, jobName, pdb.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (pdb *pipelineDB) UpdateFirstLoggedBuildID(job string, newFirstLoggedBuildID int) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return err
	}

	if dbJob.FirstLoggedBuildID > newFirstLoggedBuildID {
		return FirstLoggedBuildIDDecreasedError{
			Job:   job,
			OldID: dbJob.FirstLoggedBuildID,
			NewID: newFirstLoggedBuildID,
		}
	}

	result, err := tx.Exec(`
		UPDATE jobs
		SET first_logged_build_id = $1
		WHERE id = $2
	`, newFirstLoggedBuildID, dbJob.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) updatePausedJob(job string, pause bool) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return err
	}

	result, err := tx.Exec(`
		UPDATE jobs
		SET paused = $1
		WHERE id = $2
	`, pause, dbJob.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetJobBuilds(jobName string, page Page) ([]Build, Pagination, error) {
	var (
		err        error
		maxID      int
		minID      int
		firstBuild Build
		lastBuild  Build
		pagination Pagination

		rows *sql.Rows
	)

	query := fmt.Sprintf(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
	`)

	if page.Since == 0 && page.Until == 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY b.id DESC
			LIMIT $3
		`, query), jobName, pdb.ID, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else if page.Until != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			SELECT sub.*
			FROM (%s
					AND b.id > $3
				ORDER BY b.id ASC
				LIMIT $4
			) sub
			ORDER BY sub.id DESC
		`, query), jobName, pdb.ID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
				%s
				AND b.id < $3
			ORDER BY b.id DESC
			LIMIT $4
		`, query), jobName, pdb.ID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build, _, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return []Build{}, Pagination{}, nil
	}

	err = pdb.conn.QueryRow(`
		SELECT COALESCE(MAX(b.id), 0) as maxID,
			COALESCE(MIN(b.id), 0) as minID
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
	`, jobName, pdb.ID).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	firstBuild = builds[0]
	lastBuild = builds[len(builds)-1]

	if firstBuild.ID() < maxID {
		pagination.Previous = &Page{
			Until: firstBuild.ID(),
			Limit: page.Limit,
		}
	}

	if lastBuild.ID() > minID {
		pagination.Next = &Page{
			Since: lastBuild.ID(),
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
}

func (pdb *pipelineDB) GetAllJobBuilds(job string) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
		ORDER BY b.id DESC
	`, job, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, _, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) GetJobFinishedAndNextBuild(job string) (Build, Build, error) {
	finished, _, err := pdb.buildFactory.ScanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			INNER JOIN teams t ON b.team_id = t.id
 		WHERE j.name = $1
			AND j.pipeline_id = $2
			AND b.status NOT IN ('pending', 'started')
		ORDER BY b.id DESC
		LIMIT 1
	`, job, pdb.ID))
	if err != nil {
		return nil, nil, err
	}

	next, _, err := pdb.buildFactory.ScanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			INNER JOIN teams t ON b.team_id = t.id
 		WHERE j.name = $1
			AND j.pipeline_id = $2
			AND status IN ('pending', 'started')
		ORDER BY b.id ASC
		LIMIT 1
	`, job, pdb.ID))
	if err != nil {
		return nil, nil, err
	}

	return finished, next, nil
}

func (pdb *pipelineDB) Expose() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET public = true
		WHERE id = $1
	`, pdb.ID)
	return err
}

func (pdb *pipelineDB) Hide() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET public = false
		WHERE id = $1
	`, pdb.ID)
	return err
}

func (pdb *pipelineDB) getJobs() ([]SavedJob, error) {
	rows, err := pdb.conn.Query(`
		SELECT j.id, j.name, j.config, j.paused, j.first_logged_build_id, p.team_id
		FROM jobs j, pipelines p
		WHERE j.pipeline_id = p.id
		AND pipeline_id = $1
		AND active = true
		ORDER BY j.id ASC
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedJobs := []SavedJob{}

	for rows.Next() {
		savedJob, err := pdb.scanJob(rows)
		if err != nil {
			return nil, err
		}

		savedJobs = append(savedJobs, savedJob)
	}

	return savedJobs, nil
}

func (pdb *pipelineDB) getLastJobBuildsSatisfying(bRequirement string) (map[string]Build, error) {
	rows, err := pdb.conn.Query(`
		 SELECT `+qualifiedBuildColumns+`
		 FROM builds b, jobs j, pipelines p, teams t,
			 (
				 SELECT b.job_id AS job_id, MAX(b.id) AS id
				 FROM builds b, jobs j
				 WHERE b.job_id = j.id
					 AND `+bRequirement+`
					 AND j.pipeline_id = $1
				 GROUP BY b.job_id
			 ) max
		 WHERE b.job_id = j.id
			 AND b.id = max.id
			 AND p.id = $1
			 AND j.pipeline_id = p.id
			 AND b.team_id = t.id
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	nextBuilds := make(map[string]Build)

	for rows.Next() {
		build, scanned, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}

		if !scanned {
			return nil, errors.New("row could not be scanned")
		}

		nextBuilds[build.JobName()] = build
	}

	return nextBuilds, nil
}

func (pdb *pipelineDB) getJob(tx Tx, name string) (SavedJob, error) {
	return pdb.scanJob(tx.QueryRow(`
 	SELECT j.id, j.name, j.config, j.paused, j.first_logged_build_id, p.team_id
  	FROM jobs j, pipelines p
  	WHERE j.active = true
			AND j.pipeline_id = p.id
			AND j.name = $1
  		AND j.pipeline_id = $2
  	`, name, pdb.ID))
}

func (pdb *pipelineDB) scanJob(row scannable) (SavedJob, error) {
	var job SavedJob
	var configBlob []byte

	err := row.Scan(&job.ID, &job.Name, &configBlob, &job.Paused, &job.FirstLoggedBuildID, &job.TeamID)
	if err != nil {
		return SavedJob{}, err
	}

	job.PipelineName = pdb.Name

	var config atc.JobConfig
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedJob{}, err
	}
	job.Config = config

	return job, nil
}

func checkIfRowsUpdated(tx Tx, query string, params ...interface{}) (bool, error) {
	result, err := tx.Exec(query, params...)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rows == 0 {
		return false, nil
	}

	return true, nil
}
