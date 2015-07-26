package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/concourse/atc"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . PipelineDB

type PipelineDB interface {
	GetPipelineName() string
	ScopedName(string) string

	Pause() error
	Unpause() error
	IsPaused() (bool, error)

	Destroy() error

	GetConfig() (atc.Config, ConfigVersion, error)

	GetResource(resourceName string) (SavedResource, error)
	GetResourceHistory(resource string) ([]*VersionHistory, error)
	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	GetLatestVersionedResource(resource SavedResource) (SavedVersionedResource, error)
	EnableVersionedResource(resourceID int) error
	DisableVersionedResource(resourceID int) error
	SetResourceCheckError(resource SavedResource, err error) error

	GetJob(job string) (SavedJob, error)
	PauseJob(job string) error
	UnpauseJob(job string) error

	GetJobFinishedAndNextBuild(job string) (*Build, *Build, error)

	GetAllJobBuilds(job string) ([]Build, error)
	GetJobBuild(job string, build string) (Build, error)
	CreateJobBuild(job string) (Build, error)
	CreateJobBuildForCandidateInputs(job string) (Build, bool, error)

	UseInputsForBuild(buildID int, inputs []BuildInput) error

	GetLatestInputVersions([]atc.JobInput) ([]BuildInput, error)
	GetJobBuildForInputs(job string, inputs []BuildInput) (Build, error)
	GetNextPendingBuild(job string) (Build, error)

	GetCurrentBuild(job string) (Build, error)
	GetRunningBuildsBySerialGroup(jobName string, serialGrous []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, error)

	ScheduleBuild(buildID int, job atc.JobConfig) (bool, error)
	SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error)
	SaveBuildOutput(buildID int, vr VersionedResource) (SavedVersionedResource, error)
	GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error)
}

type pipelineDB struct {
	logger lager.Logger

	conn *sql.DB
	bus  *notificationsBus

	SavedPipeline
}

func (pdb *pipelineDB) GetPipelineName() string {
	return pdb.Name
}

func (pdb *pipelineDB) ScopedName(name string) string {
	return pdb.Name + ":" + name
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

func (pdb *pipelineDB) Destroy() error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	queries := []string{
		`
			DELETE FROM build_events
			WHERE build_id IN (
				SELECT id
				FROM builds
				WHERE job_id IN (
					SELECT id
					FROM jobs
					WHERE pipeline_id = $1
				)
			)
		`,
		`
			DELETE FROM build_outputs
			WHERE build_id IN (
				SELECT id
				FROM builds
				WHERE job_id IN (
					SELECT id
					FROM jobs
					WHERE pipeline_id = $1
				)
			)
		`,
		`
			DELETE FROM build_inputs
			WHERE build_id IN (
				SELECT id
				FROM builds
				WHERE job_id IN (
					SELECT id
					FROM jobs
					WHERE pipeline_id = $1
				)
			)
		`,
		`
			DELETE FROM jobs_serial_groups
			WHERE job_id IN (
				SELECT id
				FROM jobs
				WHERE pipeline_id = $1
			)
		`,
		`
			DELETE FROM builds
			WHERE job_id IN (
				SELECT id
				FROM jobs
				WHERE pipeline_id = $1
			)
		`,
		`
			DELETE FROM jobs
			WHERE pipeline_id = $1
		`,
		`
			DELETE FROM versioned_resources
			WHERE resource_id IN (
				SELECT id
				FROM resources
				WHERE pipeline_id = $1
			)
		`,
		`
			DELETE FROM resources
			WHERE pipeline_id = $1
		`,
		`
			DELETE FROM pipelines
			WHERE id = $1;
		`,
	}

	for _, query := range queries {
		_, err = tx.Exec(query, pdb.ID)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetConfig() (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int

	err := pdb.conn.QueryRow(`
			SELECT config, version
			FROM pipelines
			WHERE id = $1
		`, pdb.ID).Scan(&configBlob, &version)
	if err != nil {
		return atc.Config{}, 0, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, 0, err
	}

	return config, ConfigVersion(version), nil
}

func (pdb *pipelineDB) GetResource(resourceName string) (SavedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedResource{}, err
	}

	defer tx.Rollback()

	err = pdb.registerResource(tx, resourceName)
	if err != nil {
		return SavedResource{}, err
	}

	resource, err := pdb.getResource(tx, resourceName)
	if err != nil {
		return SavedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedResource{}, err
	}

	return resource, nil
}

// function moved over without tests since there were no tests before. Need to backfill later
func (pdb *pipelineDB) GetResourceHistory(resource string) ([]*VersionHistory, error) {
	hs := []*VersionHistory{}
	vhs := map[int]*VersionHistory{}

	inputHs := map[int]map[string]*JobHistory{}
	outputHs := map[int]map[string]*JobHistory{}
	seenInputs := map[int]map[int]bool{}

	dbResource, err := pdb.GetResource(resource)
	if err != nil {
		return nil, err
	}

	vrRows, err := pdb.conn.Query(`
		SELECT v.id, v.enabled, v.type, v.version, v.source, v.metadata, r.name
		FROM versioned_resources v
		INNER JOIN resources r ON v.resource_id = r.id
		WHERE v.resource_id = $1
		ORDER BY v.id DESC
	`, dbResource.ID)
	if err != nil {
		return nil, err
	}

	defer vrRows.Close()

	for vrRows.Next() {
		var svr SavedVersionedResource

		var versionString, sourceString, metadataString string

		err := vrRows.Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionString, &sourceString, &metadataString, &svr.Resource)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(sourceString), &svr.Source)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(versionString), &svr.Version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(metadataString), &svr.Metadata)
		if err != nil {
			return nil, err
		}

		vhs[svr.ID] = &VersionHistory{
			VersionedResource: svr,
		}

		hs = append(hs, vhs[svr.ID])

		inputHs[svr.ID] = map[string]*JobHistory{}
		outputHs[svr.ID] = map[string]*JobHistory{}
		seenInputs[svr.ID] = map[int]bool{}
	}

	for id, vh := range vhs {
		inRows, err := pdb.conn.Query(`
			SELECT `+qualifiedBuildColumns+`
			FROM builds b
			INNER JOIN build_inputs i ON i.build_id = b.id
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			WHERE i.versioned_resource_id = $1
			ORDER BY b.id ASC
		`, id)
		if err != nil {
			return nil, err
		}

		defer inRows.Close()

		outRows, err := pdb.conn.Query(`
			SELECT `+qualifiedBuildColumns+`
			FROM builds b
			INNER JOIN build_outputs o ON o.build_id = b.id
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			WHERE o.versioned_resource_id = $1
			ORDER BY b.id ASC
		`, id)
		if err != nil {
			return nil, err
		}

		defer outRows.Close()

		for inRows.Next() {
			inBuild, err := pdb.scanBuild(inRows)
			if err != nil {
				return nil, err
			}

			seenInputs[id][inBuild.ID] = true

			inputH, found := inputHs[id][inBuild.JobName]
			if !found {
				inputH = &JobHistory{
					JobName: inBuild.JobName,
				}

				vh.InputsTo = append(vh.InputsTo, inputH)

				inputHs[id][inBuild.JobName] = inputH
			}

			inputH.Builds = append(inputH.Builds, inBuild)
		}

		for outRows.Next() {
			outBuild, err := pdb.scanBuild(outRows)
			if err != nil {
				return nil, err
			}

			if seenInputs[id][outBuild.ID] {
				// don't show implicit outputs
				continue
			}

			outputH, found := outputHs[id][outBuild.JobName]
			if !found {
				outputH = &JobHistory{
					JobName: outBuild.JobName,
				}

				vh.OutputsOf = append(vh.OutputsOf, outputH)

				outputHs[id][outBuild.JobName] = outputH
			}

			outputH.Builds = append(outputH.Builds, outBuild)
		}
	}

	return hs, nil
}

func (pdb *pipelineDB) getResource(tx *sql.Tx, name string) (SavedResource, error) {
	var checkErr sql.NullString
	var resource SavedResource

	err := tx.QueryRow(`
			SELECT id, name, check_error, paused
			FROM resources
			WHERE name = $1
				AND pipeline_id = $2
		`, name, pdb.ID).Scan(&resource.ID, &resource.Name, &checkErr, &resource.Paused)
	if err != nil {
		return SavedResource{}, err
	}

	if checkErr.Valid {
		resource.CheckError = errors.New(checkErr.String)
	}

	resource.PipelineName = pdb.Name

	return resource, nil
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

	err = pdb.registerResource(tx, resource)
	if err != nil {
		return err
	}

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
		_, err := pdb.saveVersionedResource(tx, VersionedResource{
			Resource: config.Name,
			Type:     config.Type,
			Source:   Source(config.Source),
			Version:  Version(version),
		})
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

func (pdb *pipelineDB) DisableVersionedResource(resourceID int) error {
	rows, err := pdb.conn.Exec(`
		UPDATE versioned_resources
		SET enabled = false
		WHERE id = $1
	`, resourceID)
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

func (pdb *pipelineDB) EnableVersionedResource(resourceID int) error {
	rows, err := pdb.conn.Exec(`
		UPDATE versioned_resources
		SET enabled = true
		WHERE id = $1
	`, resourceID)
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

func (pdb *pipelineDB) GetLatestVersionedResource(resource SavedResource) (SavedVersionedResource, error) {
	var sourceBytes, versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource: resource.Name,
		},
	}

	err := pdb.conn.QueryRow(`
		SELECT id, enabled, type, source, version, metadata
		FROM versioned_resources
		WHERE resource_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, resource.ID).Scan(&svr.ID, &svr.Enabled, &svr.Type, &sourceBytes, &versionBytes, &metadataBytes)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = json.Unmarshal([]byte(sourceBytes), &svr.Source)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = json.Unmarshal([]byte(versionBytes), &svr.Version)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &svr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
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

func (pdb *pipelineDB) registerResource(tx *sql.Tx, name string) error {
	_, err := tx.Exec(`
		INSERT INTO resources (name, pipeline_id)
		SELECT $1, $2
		WHERE NOT EXISTS (
			SELECT 1 FROM resources WHERE name = $1 AND pipeline_id = $2
		)
	`, name, pdb.ID)
	return err
}

func (pdb *pipelineDB) saveVersionedResource(tx *sql.Tx, vr VersionedResource) (SavedVersionedResource, error) {
	err := pdb.registerResource(tx, vr.Resource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	savedResource, err := pdb.getResource(tx, vr.Resource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	versionJSON, err := json.Marshal(vr.Version)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	sourceJSON, err := json.Marshal(vr.Source)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	metadataJSON, err := json.Marshal(vr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	var id int
	var enabled bool

	_, err = tx.Exec(`
		INSERT INTO versioned_resources (resource_id, type, version, source, metadata)
		SELECT $1, $2, $3, $4, $5
		WHERE NOT EXISTS (
			SELECT 1
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		)
	`, savedResource.ID, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON))
	if err != nil {
		return SavedVersionedResource{}, err
	}

	// separate from above, as it conditionally inserts (can't use RETURNING)
	err = tx.QueryRow(`
		UPDATE versioned_resources
		SET source = $4, metadata = $5
		WHERE resource_id = $1
		AND type = $2
		AND version = $3
		RETURNING id, enabled
	`, savedResource.ID, vr.Type, string(versionJSON), string(sourceJSON), string(metadataJSON)).Scan(&id, &enabled)

	if err != nil {
		return SavedVersionedResource{}, err
	}

	return SavedVersionedResource{
		ID:      id,
		Enabled: enabled,

		VersionedResource: vr,
	}, nil
}

func (pdb *pipelineDB) GetJob(jobName string) (SavedJob, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedJob{}, err
	}

	defer tx.Rollback()

	err = pdb.registerJob(tx, jobName)
	if err != nil {
		return SavedJob{}, err
	}

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return SavedJob{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedJob{}, err
	}

	return dbJob, nil
}

func (pdb *pipelineDB) GetJobBuild(job string, name string) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()
	err = pdb.registerJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	build, err := pdb.scanBuild(tx.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.job_id = $1
		AND b.name = $2
	`, dbJob.ID, name))
	if err != nil {
		return Build{}, err
	}

	tx.Commit()

	return build, nil
}

func (pdb *pipelineDB) CreateJobBuildForCandidateInputs(jobName string) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, false, err
	}

	defer tx.Rollback()

	var x int
	err = tx.QueryRow(`
		SELECT 1
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
			AND b.inputs_determined = false
			AND b.status != 'errored'
	`, jobName).Scan(&x)

	if err == sql.ErrNoRows {
		build, err := pdb.createJobBuild(jobName, tx)
		if err != nil {
			return Build{}, false, err
		}

		err = tx.Commit()
		if err != nil {
			return Build{}, false, err
		}

		return build, true, nil
	}

	err = tx.Commit()

	return Build{}, false, err
}

func (pdb *pipelineDB) UseInputsForBuild(buildID int, inputs []BuildInput) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	for _, input := range inputs {
		_, err := pdb.saveBuildInput(tx, buildID, input)
		if err != nil {
			return err
		}
	}

	result, err := tx.Exec(`
		UPDATE builds b
		SET inputs_determined = true
		WHERE b.id = $1
	`, buildID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return errors.New("multiple rows affected but expected only one when determining inputs")
	}

	return tx.Commit()
}

func (pdb *pipelineDB) CreateJobBuild(jobName string) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	build, err := pdb.createJobBuild(jobName, tx)
	if err != nil {
		return Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) createJobBuild(jobName string, tx *sql.Tx) (Build, error) {
	err := pdb.registerJob(tx, jobName)
	if err != nil {
		return Build{}, err
	}

	dbJob, err := pdb.getJob(tx, jobName)

	var name string

	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE id = $1
		RETURNING build_number_seq
	`, dbJob.ID).Scan(&name)
	if err != nil {
		return Build{}, err
	}

	// We had to resort to sub-selects here because you can't paramaterize a
	// RETURNING statement in lib/pq... sorry

	build, err := pdb.scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, job_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`,
			(
				SELECT j.name
				FROM jobs j
				WHERE j.id = job_id
			),
			(
				SELECT p.name
				FROM jobs j
				INNER JOIN pipelines p ON j.pipeline_id = p.id
				WHERE j.id = job_id
			)
	`, name, dbJob.ID))
	if err != nil {
		return Build{}, err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(build.ID)))
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error) {
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

func (pdb *pipelineDB) saveBuildInput(tx *sql.Tx, buildID int, input BuildInput) (SavedVersionedResource, error) {
	svr, err := pdb.saveVersionedResource(tx, input.VersionedResource)
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
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) SaveBuildOutput(buildID int, vr VersionedResource) (SavedVersionedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	defer tx.Rollback()

	svr, err := pdb.saveVersionedResource(tx, vr)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO build_outputs (build_id, versioned_resource_id)
		VALUES ($1, $2)
	`, buildID, svr.ID)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) GetJobBuildForInputs(job string, inputs []BuildInput) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	err = pdb.registerJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, err
	}
	tx.Commit()

	from := []string{"builds b"}
	from = append(from, "jobs j")
	from = append(from, "pipelines p")
	conditions := []string{"job_id = $1"}
	conditions = append(conditions, "b.job_id = j.id")
	conditions = append(conditions, "j.pipeline_id = p.id")
	params := []interface{}{dbJob.ID}

	for i, input := range inputs {
		vr := input.VersionedResource
		dbResource, err := pdb.GetResource(vr.Resource)
		if err != nil {
			return Build{}, err
		}

		versionBytes, err := json.Marshal(vr.Version)
		if err != nil {
			return Build{}, err
		}

		var id int

		err = pdb.conn.QueryRow(`
			SELECT id
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		`, dbResource.ID, vr.Type, string(versionBytes)).Scan(&id)
		if err != nil {
			return Build{}, err
		}

		from = append(from, fmt.Sprintf("build_inputs i%d", i+1))
		params = append(params, id, input.Name)

		conditions = append(conditions,
			fmt.Sprintf("i%d.build_id = b.id", i+1),
			fmt.Sprintf("i%d.versioned_resource_id = $%d", i+1, len(params)-1),
			fmt.Sprintf("i%d.name = $%d", i+1, len(params)),
		)
	}

	return pdb.scanBuild(pdb.conn.QueryRow(fmt.Sprintf(`
		SELECT `+qualifiedBuildColumns+`
		FROM %s
		WHERE %s
		`,
		strings.Join(from, ", "),
		strings.Join(conditions, "\nAND ")),
		params...,
	))
}

func (pdb *pipelineDB) GetNextPendingBuild(job string) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, err
	}
	err = pdb.registerJob(tx, job)
	if err != nil {
		return Build{}, err
	}

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, err
	}
	tx.Commit()

	build, err := pdb.scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.job_id = $1
		AND b.status = 'pending'
		ORDER BY b.id ASC
		LIMIT 1
	`, dbJob.ID))
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := pdb.conn.Query(`
		SELECT i.name, r.name, v.type, v.source, v.version, v.metadata,
		NOT EXISTS (
			SELECT 1
			FROM build_inputs ci, builds cb
			WHERE versioned_resource_id = v.id
			AND cb.job_id = b.job_id
			AND ci.build_id = cb.id
			AND ci.build_id < b.id
		)
		FROM versioned_resources v, build_inputs i, builds b, resources r
		WHERE b.id = $1
		AND i.build_id = b.id
		AND i.versioned_resource_id = v.id
    AND r.id = v.resource_id
	`, buildID)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var inputName string
		var vr VersionedResource
		var firstOccurrence bool

		var source, version, metadata string
		err := rows.Scan(&inputName, &vr.Resource, &vr.Type, &source, &version, &metadata, &firstOccurrence)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(source), &vr.Source)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		vr.PipelineName = pdb.Name

		inputs = append(inputs, BuildInput{
			Name:              inputName,
			VersionedResource: vr,
			FirstOccurrence:   firstOccurrence,
		})
	}

	rows, err = pdb.conn.Query(`
		SELECT r.name, v.type, v.source, v.version, v.metadata
		FROM versioned_resources v, build_outputs o, builds b, resources r
		WHERE b.id = $1
		AND o.build_id = b.id
		AND o.versioned_resource_id = v.id
    AND r.id = v.resource_id
		AND NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE versioned_resource_id = v.id
			AND build_id = b.id
		)
	`, buildID)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var vr VersionedResource

		var source, version, metadata string
		err := rows.Scan(&vr.Resource, &vr.Type, &source, &version, &metadata)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(source), &vr.Source)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		outputs = append(outputs, BuildOutput{
			VersionedResource: vr,
		})
	}

	return inputs, outputs, nil
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

func (pdb *pipelineDB) GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	serialGroupNames := []interface{}{}
	refs := []string{}
	serialGroupNames = append(serialGroupNames, pdb.ID)
	for i, serialGroup := range serialGroups {
		serialGroupNames = append(serialGroupNames, serialGroup)
		refs = append(refs, fmt.Sprintf("$%d", i+2))
	}

	build, err := pdb.scanBuild(pdb.conn.QueryRow(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE b.status = 'pending'
			AND j.pipeline_id = $1
		ORDER BY b.id ASC
		LIMIT 1
	`, serialGroupNames...))

	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	serialGroupNames := []interface{}{}
	refs := []string{}
	serialGroupNames = append(serialGroupNames, pdb.ID)
	for i, serialGroup := range serialGroups {
		serialGroupNames = append(serialGroupNames, serialGroup)
		refs = append(refs, fmt.Sprintf("$%d", i+2))
	}

	rows, err := pdb.conn.Query(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE (
				b.status = 'started'
				OR
				(b.scheduled = true AND b.status = 'pending')
			)
			AND j.pipeline_id = $1
	`, serialGroupNames...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, err := pdb.scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) getBuild(buildID int) (Build, error) {
	return pdb.scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.id = $1
	`, buildID))
}

func (pdb *pipelineDB) ScheduleBuild(buildID int, jobConfig atc.JobConfig) (bool, error) {
	pipelinePaused, err := pdb.IsPaused()
	if err != nil {
		pdb.logger.Error("build-did-not-schedule", err, lager.Data{
			"reason":  "unexpected error",
			"buildID": string(buildID),
		})
		return false, err
	}

	if pipelinePaused {
		pdb.logger.Debug("build-did-not-schedule", lager.Data{
			"reason":  "pipeline-paused",
			"buildID": string(buildID),
		})
		return false, nil
	}

	build, err := pdb.getBuild(buildID)
	if err != nil {
		return false, err
	}

	// The function needs to be idempotent, that's why this isn't in CanBuildBeScheduled
	if build.Scheduled {
		return true, nil
	}

	jobService, err := NewJobService(jobConfig, pdb)
	if err != nil {
		return false, err
	}

	canBuildBeScheduled, reason, err := jobService.CanBuildBeScheduled(build)
	if err != nil {
		return false, err
	}

	if canBuildBeScheduled {
		updated, err := pdb.updateBuildToScheduled(buildID)
		if err != nil {
			return false, err
		}

		return updated, nil
	} else {
		pdb.logger.Debug("build-did-not-schedule", lager.Data{
			"reason":  reason,
			"buildID": string(buildID),
		})
		return false, nil
	}
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

func (pdb *pipelineDB) updateBuildToScheduled(buildID int) (bool, error) {
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

func (pdb *pipelineDB) GetCurrentBuild(job string) (Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
		AND j.pipeline_id = $2
		AND b.status != 'pending'
		ORDER BY b.id DESC
		LIMIT 1
	`, job, pdb.ID)
	if err != nil {
		return Build{}, err
	}

	defer rows.Close()

	if rows.Next() {
		return pdb.scanBuild(rows)
	}

	pendingRows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
		AND j.pipeline_id = $2
		AND b.status = 'pending'
		ORDER BY b.id ASC
		LIMIT 1
		`, job, pdb.ID)
	if err != nil {
		return Build{}, err
	}

	defer pendingRows.Close()

	if pendingRows.Next() {
		return pdb.scanBuild(pendingRows)
	}

	return Build{}, ErrNoBuild
}

// buckle up
func (pdb *pipelineDB) GetLatestInputVersions(inputs []atc.JobInput) ([]BuildInput, error) {
	fromAliases := []string{}
	conditions := []string{}
	params := []interface{}{}

	passedJobs := map[string]int{}

	for _, input := range inputs {
		dbResource, err := pdb.GetResource(input.Resource)
		if err != nil {
			return []BuildInput{}, err
		}

		params = append(params, dbResource.ID)
	}

	for i, input := range inputs {
		fromAliases = append(fromAliases, fmt.Sprintf("versioned_resources v%d", i+1))
		fromAliases = append(fromAliases, fmt.Sprintf("resources r%d", i+1))

		conditions = append(conditions, fmt.Sprintf("v%d.resource_id = $%d", i+1, i+1))
		conditions = append(conditions, fmt.Sprintf("v%d.resource_id = r%d.id", i+1, i+1))

		for _, name := range input.Passed {
			idx, found := passedJobs[name]
			if !found {
				idx = len(passedJobs)
				passedJobs[name] = idx

				fromAliases = append(fromAliases, fmt.Sprintf("builds b%d", idx+1))

				conditions = append(conditions, fmt.Sprintf("b%d.job_id = $%d", idx+1, idx+len(inputs)+1))

				dbJob, err := pdb.GetJob(name)
				if err != nil {
					return []BuildInput{}, err
				}

				// add job id to params
				params = append(params, dbJob.ID)
			}

			fromAliases = append(fromAliases, fmt.Sprintf("build_outputs v%db%d", i+1, idx+1))

			conditions = append(conditions, fmt.Sprintf("v%db%d.versioned_resource_id = v%d.id", i+1, idx+1, i+1))

			conditions = append(conditions, fmt.Sprintf("v%db%d.build_id = b%d.id", i+1, idx+1, idx+1))
		}
	}

	buildInputs := []BuildInput{}

	for i, input := range inputs {
		svr := SavedVersionedResource{
			Enabled: true, // this is inherent with the following query
		}

		var source, version, metadata string

		err := pdb.conn.QueryRow(fmt.Sprintf(
			`
				SELECT v%[1]d.id, r%[1]d.name, v%[1]d.type, v%[1]d.source, v%[1]d.version, v%[1]d.metadata
				FROM %s
				WHERE %s
				AND v%[1]d.enabled
				ORDER BY v%[1]d.id DESC
				LIMIT 1
			`,
			i+1,
			strings.Join(fromAliases, ", "),
			strings.Join(conditions, "\nAND "),
		), params...).Scan(&svr.ID, &svr.Resource, &svr.Type, &source, &version, &metadata)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil, ErrNoVersions
			}

			return nil, err
		}

		params = append(params, svr.ID)
		conditions = append(conditions, fmt.Sprintf("v%d.id = $%d", i+1, len(params)))

		err = json.Unmarshal([]byte(source), &svr.Source)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(version), &svr.Version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(metadata), &svr.Metadata)
		if err != nil {
			return nil, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:              input.Name,
			VersionedResource: svr.VersionedResource,
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

func (pdb *pipelineDB) updatePausedJob(job string, pause bool) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = pdb.registerJob(tx, job)
	if err != nil {
		return err
	}

	dbJob, err := pdb.getJob(tx, job)

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

func (pdb *pipelineDB) GetAllJobBuilds(job string) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
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
		build, err := pdb.scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) GetJobFinishedAndNextBuild(job string) (*Build, *Build, error) {
	var finished *Build
	var next *Build

	finishedBuild, err := pdb.scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
 		WHERE j.name = $1
		AND j.pipeline_id = $2
	 	AND b.status NOT IN ('pending', 'started')
		ORDER BY b.id DESC
		LIMIT 1
	`, job, pdb.ID))
	if err == nil {
		finished = &finishedBuild
	} else if err != nil && err != ErrNoBuild {
		return nil, nil, err
	}

	nextBuild, err := pdb.scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
 		WHERE j.name = $1
		AND j.pipeline_id = $2
		AND status IN ('pending', 'started')
		ORDER BY b.id ASC
		LIMIT 1
	`, job, pdb.ID))
	if err == nil {
		next = &nextBuild
	} else if err != nil && err != ErrNoBuild {
		return nil, nil, err
	}

	return finished, next, nil
}

func (pdb *pipelineDB) registerJob(tx *sql.Tx, name string) error {
	_, err := tx.Exec(`
  		INSERT INTO jobs (name, pipeline_id)
  		SELECT $1, $2
  		WHERE NOT EXISTS (
  			SELECT 1 FROM jobs WHERE name = $1 AND pipeline_id = $2
  		)
  	`, name, pdb.ID)
	return err
}

func (pdb *pipelineDB) getJob(tx *sql.Tx, name string) (SavedJob, error) {
	var job SavedJob

	err := tx.QueryRow(`
  	SELECT id, name, paused
  	FROM jobs
  	WHERE name = $1
  		AND pipeline_id = $2
  `, name, pdb.ID).Scan(&job.ID, &job.Name, &job.Paused)
	if err != nil {
		return SavedJob{}, err
	}

	job.PipelineName = pdb.Name

	return job, nil
}

func (pdb *pipelineDB) getJobByID(id int) (SavedJob, error) {
	var job SavedJob

	err := pdb.conn.QueryRow(`
		SELECT id, name, paused
		FROM jobs
		WHERE id = $1
  `, id).Scan(&job.ID, &job.Name, &job.Paused)
	if err != nil {
		return SavedJob{}, err
	}

	job.PipelineName = pdb.Name

	return job, nil
}

func (pdb *pipelineDB) scanBuild(row scannable) (Build, error) {
	var id int
	var name string
	var jobID int
	var status string
	var scheduled bool
	var engine, engineMetadata, jobName, pipelineName sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime

	err := row.Scan(&id, &name, &jobID, &status, &scheduled, &engine, &engineMetadata, &startTime, &endTime, &jobName, &pipelineName)
	if err != nil {
		if err == sql.ErrNoRows {
			return Build{}, ErrNoBuild
		}

		return Build{}, err
	}

	build := Build{
		ID:           id,
		Name:         name,
		JobID:        jobID,
		JobName:      jobName.String,
		PipelineName: pipelineName.String,
		Status:       Status(status),
		Scheduled:    scheduled,

		Engine:         engine.String,
		EngineMetadata: engineMetadata.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
	}

	if err != nil {
		return Build{}, err
	}

	return build, nil
}
