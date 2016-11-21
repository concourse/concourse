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
	UpdateName(string) error
	Destroy() error

	AcquireSchedulingLock(lager.Logger, time.Duration) (lock.Lock, bool, error)

	GetResource(resourceName string) (SavedResource, bool, error)
	GetResources() ([]SavedResource, bool, error)
	GetResourceType(resourceTypeName string) (SavedResourceType, bool, error)
	GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error)

	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	SaveResourceTypeVersion(atc.ResourceType, atc.Version) error
	GetLatestVersionedResource(resourceName string) (SavedVersionedResource, bool, error)
	GetLatestEnabledVersionedResource(resourceName string) (SavedVersionedResource, bool, error)
	EnableVersionedResource(versionedResourceID int) error
	DisableVersionedResource(versionedResourceID int) error
	SetResourceCheckError(resource SavedResource, err error) error
	AcquireResourceCheckingLock(logger lager.Logger, resource SavedResource, length time.Duration, immediate bool) (lock.Lock, bool, error)
	AcquireResourceTypeCheckingLock(logger lager.Logger, resourceType SavedResourceType, length time.Duration, immediate bool) (lock.Lock, bool, error)

	GetJobs() ([]SavedJob, error)
	GetJob(job string) (SavedJob, bool, error)
	PauseJob(job string) error
	UnpauseJob(job string) error
	SetMaxInFlightReached(string, bool) error
	UpdateFirstLoggedBuildID(job string, newFirstLoggedBuildID int) error

	GetJobFinishedAndNextBuild(job string) (Build, Build, error)

	GetJobBuilds(job string, page Page) ([]Build, Pagination, error)
	GetAllJobBuilds(job string) ([]Build, error)

	GetJobBuild(job string, build string) (Build, bool, error)
	CreateJobBuild(job string) (Build, error)
	EnsurePendingBuildExists(jobName string) error
	GetPendingBuildsForJob(jobName string) ([]Build, error)
	GetAllPendingBuilds() (map[string][]Build, error)
	UseInputsForBuild(buildID int, inputs []BuildInput) error

	LoadVersionsDB() (*algorithm.VersionsDB, error)
	GetVersionedResourceByVersion(atcVersion atc.Version, resourceName string) (SavedVersionedResource, bool, error)
	SaveIndependentInputMapping(inputMapping algorithm.InputMapping, jobName string) error
	GetIndependentBuildInputs(jobName string) ([]BuildInput, error)
	SaveNextInputMapping(inputMapping algorithm.InputMapping, jobName string) error
	GetNextBuildInputs(jobName string) ([]BuildInput, bool, error)
	DeleteNextInputMapping(jobName string) error

	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error)

	UpdateBuildToScheduled(buildID int) (bool, error)
	SaveInput(buildID int, input BuildInput) (SavedVersionedResource, error)
	SaveOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error)
	GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error)
	GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error)

	GetDashboard() (Dashboard, atc.GroupConfigs, error)

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

func (pdb *pipelineDB) UpdateName(newName string) error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET name = $1
		WHERE id = $2
	`, newName, pdb.ID)
	return err
}

func (pdb *pipelineDB) Destroy() error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(fmt.Sprintf(`
		DROP TABLE pipeline_build_events_%d
	`, pdb.ID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM pipelines WHERE id = $1;
	`, pdb.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
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

func (pdb *pipelineDB) GetResources() ([]SavedResource, bool, error) {
	rows, err := pdb.conn.Query(`
			SELECT id, name, config, check_error, paused
			FROM resources
			WHERE pipeline_id = $1
				AND active = true
		`, pdb.ID)

	if err != nil {
		return nil, false, err
	}

	defer rows.Close()

	savedResources := []SavedResource{}

	for rows.Next() {
		savedResource, found, err := pdb.scanResource(rows)
		if err != nil {
			return nil, false, err
		}

		if !found {
			return nil, false, errors.New("resource-not-found")
		}

		savedResources = append(savedResources, savedResource)
	}

	return savedResources, true, nil
}

func (pdb *pipelineDB) AcquireResourceCheckingLock(logger lager.Logger, resource SavedResource, interval time.Duration, immediate bool) (lock.Lock, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	params := []interface{}{resource.Name, pdb.ID}

	condition := ""
	if !immediate {
		condition = "AND now() - last_checked > ($3 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resources
		SET last_checked = now()
		WHERE name = $1
			AND pipeline_id = $2
	`+condition, params...)
	if err != nil {
		return nil, false, err
	}

	if !updated {
		return nil, false, nil
	}

	lock := pdb.lockFactory.NewLock(
		logger.Session("lock", lager.Data{
			"resource": resource.Name,
		}),
		lock.NewResourceCheckingLockID(resource.ID),
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

func (pdb *pipelineDB) AcquireResourceTypeCheckingLock(logger lager.Logger, resourceType SavedResourceType, interval time.Duration, immediate bool) (lock.Lock, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	params := []interface{}{resourceType.Name, pdb.ID}

	condition := ""
	if !immediate {
		condition = "AND now() - last_checked > ($3 || ' SECONDS')::INTERVAL"
		params = append(params, interval.Seconds())
	}

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE resource_types
		SET last_checked = now()
		WHERE name = $1
			AND pipeline_id = $2
	`+condition, params...)
	if err != nil {
		return nil, false, err
	}

	if !updated {
		return nil, false, nil
	}

	lock := pdb.lockFactory.NewLock(
		logger.Session("lock", lager.Data{
			"resource-type": resourceType.Name,
		}),
		lock.NewResourceTypeCheckingLockID(resourceType.ID),
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

func (pdb *pipelineDB) GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error) {
	dbResource, found, err := pdb.GetResource(resourceName)
	if err != nil {
		return []SavedVersionedResource{}, Pagination{}, false, err
	}

	if !found {
		return []SavedVersionedResource{}, Pagination{}, false, nil
	}

	query := `
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, r.name, v.check_order
		FROM versioned_resources v
		INNER JOIN resources r ON v.resource_id = r.id
		WHERE v.resource_id = $1
	`

	var rows *sql.Rows
	if page.Until != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND v.check_order > (SELECT check_order FROM versioned_resources WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), dbResource.ID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.Since != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
				AND v.check_order < (SELECT check_order FROM versioned_resources WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), dbResource.ID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.To != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND v.check_order >= (SELECT check_order FROM versioned_resources WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), dbResource.ID, page.To, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.From != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
				AND v.check_order <= (SELECT check_order FROM versioned_resources WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), dbResource.ID, page.From, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY v.check_order DESC
			LIMIT $2
		`, query), dbResource.ID, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	}

	defer rows.Close()

	savedVersionedResources := make([]SavedVersionedResource, 0)
	for rows.Next() {
		var savedVersionedResource SavedVersionedResource

		var versionString, metadataString string

		err := rows.Scan(
			&savedVersionedResource.ID,
			&savedVersionedResource.Enabled,
			&savedVersionedResource.Type,
			&versionString,
			&metadataString,
			&savedVersionedResource.Resource,
			&savedVersionedResource.CheckOrder,
		)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		err = json.Unmarshal([]byte(versionString), &savedVersionedResource.Version)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		err = json.Unmarshal([]byte(metadataString), &savedVersionedResource.Metadata)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		savedVersionedResource.PipelineID = pdb.GetPipelineID()

		savedVersionedResources = append(savedVersionedResources, savedVersionedResource)
	}

	if len(savedVersionedResources) == 0 {
		return []SavedVersionedResource{}, Pagination{}, true, nil
	}

	var minCheckOrder int
	var maxCheckOrder int

	err = pdb.conn.QueryRow(`
		SELECT COALESCE(MAX(v.check_order), 0) as maxCheckOrder,
			COALESCE(MIN(v.check_order), 0) as minCheckOrder
		FROM versioned_resources v
		WHERE v.resource_id = $1
	`, dbResource.ID).Scan(&maxCheckOrder, &minCheckOrder)
	if err != nil {
		return nil, Pagination{}, false, err
	}

	firstSavedVersionedResource := savedVersionedResources[0]
	lastSavedVersionedResource := savedVersionedResources[len(savedVersionedResources)-1]

	var pagination Pagination

	if firstSavedVersionedResource.CheckOrder < maxCheckOrder {
		pagination.Previous = &Page{
			Until: firstSavedVersionedResource.ID,
			Limit: page.Limit,
		}
	}

	if lastSavedVersionedResource.CheckOrder > minCheckOrder {
		pagination.Next = &Page{
			Since: lastSavedVersionedResource.ID,
			Limit: page.Limit,
		}
	}

	return savedVersionedResources, pagination, true, nil
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

func (pdb *pipelineDB) PauseResource(resource string) error {
	return pdb.updatePaused(resource, true)
}

func (pdb *pipelineDB) UnpauseResource(resource string) error {
	return pdb.updatePaused(resource, false)
}

func (pdb *pipelineDB) updatePaused(resource string, pause bool) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE resources
		SET paused = $1
		WHERE name = $2
			AND pipeline_id = $3
	`, pause, resource, pdb.ID)
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

func (pdb *pipelineDB) SaveResourceVersions(config atc.ResourceConfig, versions []atc.Version) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	for _, version := range versions {
		vr := VersionedResource{
			Resource: config.Name,
			Type:     config.Type,
			Version:  Version(version),
		}

		versionJSON, err := json.Marshal(vr.Version)
		if err != nil {
			return err
		}

		savedResource, found, err := pdb.getResource(tx, vr.Resource)
		if err != nil {
			return err
		}

		if !found {
			return ResourceNotFoundError{Name: vr.Resource}
		}

		_, _, err = pdb.saveVersionedResource(tx, savedResource, vr)
		if err != nil {
			return err
		}

		err = pdb.incrementCheckOrderWhenNewerVersion(tx, savedResource.ID, vr.Type, string(versionJSON))
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
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

func (pdb *pipelineDB) GetLatestEnabledVersionedResource(resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource:   resourceName,
			PipelineID: pdb.GetPipelineID(),
		},
	}

	err := pdb.conn.QueryRow(`
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, v.modified_time
		FROM versioned_resources v, resources r
		WHERE v.resource_id = r.id
			AND r.name = $1
			AND enabled = true
			AND r.pipeline_id = $2
		ORDER BY check_order DESC
		LIMIT 1
	`, resourceName, pdb.ID).Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.ModifiedTime)
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

func (pdb *pipelineDB) GetLatestVersionedResource(resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource:   resourceName,
			PipelineID: pdb.ID,
		},
	}

	err := pdb.conn.QueryRow(`
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, v.modified_time, v.check_order
		FROM versioned_resources v, resources r
		WHERE v.resource_id = r.id
			AND r.name = $1
			AND r.pipeline_id = $2
		ORDER BY check_order DESC
		LIMIT 1
	`, resourceName, pdb.ID).Scan(
		&svr.ID,
		&svr.Enabled,
		&svr.Type,
		&versionBytes,
		&metadataBytes,
		&svr.ModifiedTime,
		&svr.CheckOrder,
	)
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

func (pdb *pipelineDB) saveVersionedResource(tx Tx, savedResource SavedResource, vr VersionedResource) (SavedVersionedResource, bool, error) {
	versionJSON, err := json.Marshal(vr.Version)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	metadataJSON, err := json.Marshal(vr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	var id int
	var enabled bool
	var modified_time time.Time
	var check_order int

	result, err := tx.Exec(`
		INSERT INTO versioned_resources (resource_id, type, version, metadata, modified_time)
		SELECT $1, $2, $3, $4, now()
		WHERE NOT EXISTS (
			SELECT 1
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		)
	`, savedResource.ID, vr.Type, string(versionJSON), string(metadataJSON))

	var rowsAffected int64
	if err == nil {
		rowsAffected, err = result.RowsAffected()
		if err != nil {
			return SavedVersionedResource{}, false, err
		}
	} else {
		err = swallowUniqueViolation(err)
		if err != nil {
			return SavedVersionedResource{}, false, err
		}
	}

	var savedMetadata string

	// separate from above, as it conditionally inserts (can't use RETURNING)
	if len(vr.Metadata) > 0 {
		err = tx.QueryRow(`
			UPDATE versioned_resources
			SET metadata = $4, modified_time = now()
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
			RETURNING id, enabled, metadata, modified_time, check_order
		`, savedResource.ID, vr.Type, string(versionJSON), string(metadataJSON)).Scan(&id, &enabled, &savedMetadata, &modified_time, &check_order)
	} else {
		err = tx.QueryRow(`
			SELECT id, enabled, metadata, modified_time, check_order
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		`, savedResource.ID, vr.Type, string(versionJSON)).Scan(&id, &enabled, &savedMetadata, &modified_time, &check_order)
	}
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(savedMetadata), &vr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	created := rowsAffected != 0
	return SavedVersionedResource{
		ID:           id,
		Enabled:      enabled,
		ModifiedTime: modified_time,

		VersionedResource: vr,
		CheckOrder:        check_order,
	}, created, nil
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

func (pdb *pipelineDB) UseInputsForBuild(buildID int, inputs []BuildInput) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		DELETE FROM build_inputs
		WHERE build_id = $1
	`, buildID)
	if err != nil {
		return err
	}

	for _, input := range inputs {
		_, err := pdb.saveBuildInput(tx, buildID, input)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
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

func (pdb *pipelineDB) EnsurePendingBuildExists(jobName string) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	buildName, jobID, err := getNewBuildNameForJob(tx, jobName, pdb.ID)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		INSERT INTO builds (name, job_id, team_id, status)
		SELECT $1, $2, $3, 'pending'
		WHERE NOT EXISTS
			(SELECT id FROM builds WHERE job_id = $2 AND status = 'pending')
		RETURNING id
	`, buildName, jobID, pdb.SavedPipeline.TeamID)
	if err != nil {
		return err
	}

	defer rows.Close()

	if rows.Next() {
		var buildID int
		err := rows.Scan(&buildID)
		if err != nil {
			return err
		}

		rows.Close()

		err = createBuildEventSeq(tx, buildID)
		if err != nil {
			return err
		}

		return tx.Commit()
	}

	return nil
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

func (pdb *pipelineDB) SaveInput(buildID int, input BuildInput) (SavedVersionedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	defer tx.Rollback()

	svr, err := pdb.saveBuildInput(tx, buildID, input)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) saveBuildInput(tx Tx, buildID int, input BuildInput) (SavedVersionedResource, error) {
	savedResource, found, err := pdb.getResource(tx, input.VersionedResource.Resource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	if !found {
		return SavedVersionedResource{}, ResourceNotFoundError{Name: input.VersionedResource.Resource}
	}

	svr, _, err := pdb.saveVersionedResource(tx, savedResource, input.VersionedResource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO build_inputs (build_id, versioned_resource_id, name)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE build_id = $1
			AND versioned_resource_id = $2
			AND name = $3
		)
	`, buildID, svr.ID, input.Name)

	err = swallowUniqueViolation(err)

	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) SaveOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	defer tx.Rollback()

	savedResource, found, err := pdb.getResource(tx, vr.Resource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	if !found {
		return SavedVersionedResource{}, ResourceNotFoundError{Name: vr.Resource}
	}

	svr, created, err := pdb.saveVersionedResource(tx, savedResource, vr)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	if created {
		versionJSON, err := json.Marshal(vr.Version)
		if err != nil {
			return SavedVersionedResource{}, err
		}

		err = pdb.incrementCheckOrderWhenNewerVersion(tx, savedResource.ID, vr.Type, string(versionJSON))
		if err != nil {
			return SavedVersionedResource{}, err
		}
	}

	_, err = tx.Exec(`
		INSERT INTO build_outputs (build_id, versioned_resource_id, explicit)
		VALUES ($1, $2, $3)
	`, buildID, svr.ID, explicit)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) GetPendingBuildsForJob(jobName string) ([]Build, error) {
	builds := []Build{}

	tx, err := pdb.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		WHERE b.job_id = $1
		AND b.status = 'pending'
		ORDER BY b.id ASC
	`, dbJob.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		build, found, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		builds = append(builds, build)
	}

	return builds, nil
}

func (pdb *pipelineDB) GetAllPendingBuilds() (map[string][]Build, error) {
	builds := map[string][]Build{}

	rows, err := pdb.conn.Query(`
		SELECT b.id, b.name, b.job_id, b.team_id, b.status, b.manually_triggered, b.scheduled, b.engine, b.engine_metadata, b.start_time, b.end_time, b.reap_time, j.name as job_name, p.id as pipeline_id, p.name as pipeline_name, t.name as team_name
		FROM builds b
		JOIN jobs j ON b.job_id = j.id
		JOIN pipelines p ON j.pipeline_id = p.id
		JOIN teams t ON b.team_id = t.id
		WHERE b.status = 'pending'
		AND j.active = true
		AND p.id = $1
	  ORDER BY b.id
	`, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		build, found, err := pdb.buildFactory.ScanBuild(rows)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		builds[build.JobName()] = append(builds[build.JobName()], build)
	}

	return builds, nil
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

func (pdb *pipelineDB) getLatestModifiedTime() (time.Time, error) {
	var max_modified_time time.Time

	err := pdb.conn.QueryRow(`
	SELECT
		CASE
			WHEN bo_max > vr_max AND bo_max > bi_max THEN bo_max
			WHEN bi_max > vr_max THEN bi_max
			ELSE vr_max
		END
	FROM
		(
			SELECT COALESCE(MAX(bo.modified_time), 'epoch') as bo_max
			FROM build_outputs bo
			LEFT OUTER JOIN versioned_resources v ON v.id = bo.versioned_resource_id
			LEFT OUTER JOIN resources r ON r.id = v.resource_id
			WHERE r.pipeline_id = $1
		) bo,
		(
			SELECT COALESCE(MAX(bi.modified_time), 'epoch') as bi_max
			FROM build_inputs bi
			LEFT OUTER JOIN versioned_resources v ON v.id = bi.versioned_resource_id
			LEFT OUTER JOIN resources r ON r.id = v.resource_id
			WHERE r.pipeline_id = $1
		) bi,
		(
			SELECT COALESCE(MAX(vr.modified_time), 'epoch') as vr_max
			FROM versioned_resources vr
			LEFT OUTER JOIN resources r ON r.id = vr.resource_id
			WHERE r.pipeline_id = $1
		) vr
	`, pdb.ID).Scan(&max_modified_time)

	return max_modified_time, err
}

func (pdb *pipelineDB) LoadVersionsDB() (*algorithm.VersionsDB, error) {
	latestModifiedTime, err := pdb.getLatestModifiedTime()
	if err != nil {
		return nil, err
	}

	if pdb.versionsDB != nil && pdb.versionsDB.CachedAt.Equal(latestModifiedTime) {
		return pdb.versionsDB, nil
	}

	db := &algorithm.VersionsDB{
		BuildOutputs:     []algorithm.BuildOutput{},
		BuildInputs:      []algorithm.BuildInput{},
		ResourceVersions: []algorithm.ResourceVersion{},
		JobIDs:           map[string]int{},
		ResourceIDs:      map[string]int{},
		CachedAt:         latestModifiedTime,
	}

	rows, err := pdb.conn.Query(`
    SELECT v.id, v.check_order, r.id, o.build_id, j.id
    FROM build_outputs o, builds b, versioned_resources v, jobs j, resources r
    WHERE v.id = o.versioned_resource_id
    AND b.id = o.build_id
    AND j.id = b.job_id
    AND r.id = v.resource_id
    AND v.enabled
		AND b.status = 'succeeded'
		AND r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output algorithm.BuildOutput
		err := rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID, &output.BuildID, &output.JobID)
		if err != nil {
			return nil, err
		}

		output.ResourceVersion.CheckOrder = output.CheckOrder

		db.BuildOutputs = append(db.BuildOutputs, output)
	}

	rows, err = pdb.conn.Query(`
    SELECT v.id, v.check_order, r.id, i.build_id, i.name, j.id
    FROM build_inputs i, builds b, versioned_resources v, jobs j, resources r
    WHERE v.id = i.versioned_resource_id
    AND b.id = i.build_id
    AND j.id = b.job_id
    AND r.id = v.resource_id
    AND v.enabled
		AND r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var input algorithm.BuildInput
		err := rows.Scan(&input.VersionID, &input.CheckOrder, &input.ResourceID, &input.BuildID, &input.InputName, &input.JobID)
		if err != nil {
			return nil, err
		}

		input.ResourceVersion.CheckOrder = input.CheckOrder

		db.BuildInputs = append(db.BuildInputs, input)
	}

	rows, err = pdb.conn.Query(`
    SELECT v.id, v.check_order, r.id
    FROM versioned_resources v, resources r
    WHERE r.id = v.resource_id
    AND v.enabled
		AND r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output algorithm.ResourceVersion
		err := rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID)
		if err != nil {
			return nil, err
		}

		db.ResourceVersions = append(db.ResourceVersions, output)
	}

	rows, err = pdb.conn.Query(`
    SELECT j.name, j.id
    FROM jobs j
    WHERE j.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.JobIDs[name] = id
	}

	rows, err = pdb.conn.Query(`
    SELECT r.name, r.id
    FROM resources r
    WHERE r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.ResourceIDs[name] = id
	}

	pdb.versionsDB = db

	return db, nil
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

func (pdb *pipelineDB) GetIndependentBuildInputs(jobName string) ([]BuildInput, error) {
	return pdb.getJobBuildInputs("independent_build_inputs", jobName)
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

func (pdb *pipelineDB) GetJobs() ([]SavedJob, error) {
	return pdb.getJobs()
}

func (pdb *pipelineDB) GetDashboard() (Dashboard, atc.GroupConfigs, error) {
	dashboard := Dashboard{}

	savedJobs, err := pdb.getJobs()
	if err != nil {
		return nil, nil, err
	}

	startedBuilds, err := pdb.getLastJobBuildsSatisfying("b.status = 'started'")
	if err != nil {
		return nil, nil, err
	}

	pendingBuilds, err := pdb.getLastJobBuildsSatisfying("b.status = 'pending'")
	if err != nil {
		return nil, nil, err
	}

	finishedBuilds, err := pdb.getLastJobBuildsSatisfying("b.status NOT IN ('pending', 'started')")
	if err != nil {
		return nil, nil, err
	}

	for _, job := range savedJobs {
		dashboardJob := DashboardJob{
			Job: job,
		}

		if startedBuild, found := startedBuilds[job.Name]; found {
			dashboardJob.NextBuild = startedBuild
		} else if pendingBuild, found := pendingBuilds[job.Name]; found {
			dashboardJob.NextBuild = pendingBuild
		}

		if finishedBuild, found := finishedBuilds[job.Name]; found {
			dashboardJob.FinishedBuild = finishedBuild
		}

		dashboard = append(dashboard, dashboardJob)
	}

	return dashboard, pdb.SavedPipeline.Config.Groups, nil
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
