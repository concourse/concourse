package dbng

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . Pipeline

type Pipeline interface {
	ID() int
	Name() string
	TeamID() int
	TeamName() string
	ConfigVersion() ConfigVersion
	Config() atc.Config
	Paused() bool
	Public() bool

	Reload() (bool, error)

	SaveJob(job atc.JobConfig) error
	CreateJobBuild(jobName string) (Build, error)
	NextBuildInputs(jobName string) ([]BuildInput, bool, error)

	SetResourceCheckError(Resource, error) error

	AcquireResourceCheckingLockWithIntervalCheck(
		logger lager.Logger,
		resource Resource,
		resourceTypes atc.VersionedResourceTypes,
		length time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	LoadVersionsDB() (*algorithm.VersionsDB, error)

	Resource(name string) (Resource, bool, error)
	Resources() ([]Resource, error)

	ResourceTypes() ([]ResourceType, error)
	ResourceType(name string) (ResourceType, bool, error)

	Job(name string) (Job, bool, error)

	Hide() error
	Destroy() error
	Expose() error
	Pause() error
	Unpause() error
	Rename(string) error
}

type pipeline struct {
	id            int
	name          string
	teamID        int
	teamName      string
	configVersion ConfigVersion
	config        atc.Config
	paused        bool
	public        bool

	conn        Conn
	lockFactory lock.LockFactory
}

//ConfigVersion is a sequence identifier used for compare-and-swap
type ConfigVersion int

type PipelinePausedState string

var pipelinesQuery = psql.Select(`
		p.id,
		p.name,
		p.version,
		p.team_id,
		t.name,
		p.config,
		p.paused,
		p.public
	`).
	From("pipelines p").
	LeftJoin("teams t ON p.team_id = t.id")

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

func newPipeline(conn Conn, lockFactory lock.LockFactory) *pipeline {
	return &pipeline{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (p *pipeline) ID() int                      { return p.id }
func (p *pipeline) Name() string                 { return p.name }
func (p *pipeline) TeamID() int                  { return p.teamID }
func (p *pipeline) TeamName() string             { return p.teamName }
func (p *pipeline) ConfigVersion() ConfigVersion { return p.configVersion }
func (p *pipeline) Config() atc.Config           { return p.config }
func (p *pipeline) Paused() bool                 { return p.paused }
func (p *pipeline) Public() bool                 { return p.public }

func (p *pipeline) Reload() (bool, error) {
	row := pipelinesQuery.Where(sq.Eq{"p.id": p.id}).
		RunWith(p.conn).
		QueryRow()

	err := scanPipeline(p, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (p *pipeline) CreateJobBuild(jobName string) (Build, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	buildName, jobID, err := getNewBuildNameForJob(tx, jobName, p.id)
	if err != nil {
		return nil, err
	}

	var buildID int
	err = psql.Insert("builds").
		Columns("name", "job_id", "team_id", "status", "manually_triggered").
		Values(buildName, jobID, p.teamID, "pending", true).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		return nil, err
	}

	build := &build{conn: p.conn, lockFactory: p.lockFactory}
	err = scanBuild(build, buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(tx).
		QueryRow(),
	)
	if err != nil {
		return nil, err
	}

	err = createBuildEventSeq(tx, buildID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (p *pipeline) NextBuildInputs(jobName string) ([]BuildInput, bool, error) {
	var found bool
	err := psql.Select("inputs_determined").
		From("jobs").
		Where(sq.Eq{
		"name":        jobName,
		"pipeline_id": p.id,
	}).
		RunWith(p.conn).
		QueryRow().
		Scan(&found)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// there is a possible race condition where found is true at first but the
	// inputs are deleted by the time we get here
	buildInputs, err := p.getJobBuildInputs("next_build_inputs", jobName)
	return buildInputs, true, err
}

func (p *pipeline) SetResourceCheckError(resource Resource, cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resources").
			Set("check_error", "NULL").
			Where(sq.Eq{"id": resource.ID()}).
			RunWith(p.conn).
			Exec()
	} else {
		_, err = psql.Update("resources").
			Set("check_error", cause.Error()).
			Where(sq.Eq{"id": resource.ID()}).
			RunWith(p.conn).
			Exec()
	}

	return err
}

func (p *pipeline) Resource(name string) (Resource, bool, error) {
	row := resourcesQuery.Where(sq.Eq{
		"r.pipeline_id": p.id,
		"r.name":        name,
	}).RunWith(p.conn).QueryRow()

	resource := &resource{conn: p.conn}
	err := scanResource(resource, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return resource, true, nil

}

func (p *pipeline) Resources() ([]Resource, error) {
	rows, err := resourcesQuery.Where(sq.Eq{"p.pipeline_id": p.id}).RunWith(p.conn).Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resources := []Resource{}

	for rows.Next() {
		newResource := &resource{conn: p.conn}
		err := scanResource(newResource, rows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, newResource)
	}

	return resources, nil
}

func (p *pipeline) SaveJob(job atc.JobConfig) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return safeCreateOrUpdate(
		p.conn,
		func(tx Tx) (sql.Result, error) {
			return psql.Insert("jobs").
				Columns("name", "pipeline_id", "config", "active").
				Values(job.Name, p.id, configPayload, true).
				RunWith(tx).
				Exec()
		},
		func(tx Tx) (sql.Result, error) {
			return psql.Update("jobs").
				Set("config", configPayload).
				Set("active", true).
				Where(sq.Eq{
				"name":        job.Name,
				"pipeline_id": p.id,
			}).
				RunWith(tx).
				Exec()
		},
	)
}

func (p *pipeline) ResourceTypes() ([]ResourceType, error) {
	rows, err := resourceTypesQuery.Where(sq.Eq{"pipeline_id": p.id}).RunWith(p.conn).Query()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resourceTypes := []ResourceType{}

	for rows.Next() {
		resourceType := &resourceType{conn: p.conn}
		err := scanResourceType(resourceType, rows)
		if err != nil {
			return nil, err
		}

		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (p *pipeline) ResourceType(name string) (ResourceType, bool, error) {
	row := resourceTypesQuery.Where(sq.Eq{
		"pipeline_id": p.id,
		"name":        name,
	}).RunWith(p.conn).QueryRow()

	resourceType := &resourceType{conn: p.conn}
	err := scanResourceType(resourceType, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return resourceType, true, nil
}

func (p *pipeline) Job(name string) (Job, bool, error) {
	row := jobQuery.Where(sq.Eq{
		"j.pipeline_id": p.id,
		"j.name":        name,
		"j.active":      true,
	}).RunWith(p.conn).QueryRow()

	job := &job{conn: p.conn}
	err := scanJob(job, row)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return job, true, nil
}

func (p *pipeline) Pause() error {
	_, err := psql.Update("pipelines").
		Set("paused", true).
		Where(sq.Eq{
		"id": p.id,
	}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) Unpause() error {
	_, err := psql.Update("pipelines").
		Set("paused", false).
		Where(sq.Eq{
		"id": p.id,
	}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) Hide() error {
	_, err := psql.Update("pipelines").
		Set("public", false).
		Where(sq.Eq{
		"id": p.id,
	}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) Expose() error {
	_, err := psql.Update("pipelines").
		Set("public", true).
		Where(sq.Eq{
		"id": p.id,
	}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) Rename(name string) error {
	_, err := psql.Update("pipelines").
		Set("name", name).
		Where(sq.Eq{
		"id": p.id,
	}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) Destroy() error {
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(fmt.Sprintf(`
		DROP TABLE pipeline_build_events_%d
	`, p.id))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM pipelines WHERE id = $1;
	`, p.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (p *pipeline) LoadVersionsDB() (*algorithm.VersionsDB, error) {
	db := &algorithm.VersionsDB{
		BuildOutputs:     []algorithm.BuildOutput{},
		BuildInputs:      []algorithm.BuildInput{},
		ResourceVersions: []algorithm.ResourceVersion{},
		JobIDs:           map[string]int{},
		ResourceIDs:      map[string]int{},
	}
	rows, err := psql.Select("v.id, v.check_order, r.id, o.build_id, j.id").
		From("build_outputs o, builds b, versioned_resources v, jobs j, resources r").
		Where(sq.Eq{
		"v.id":          "o.versioned_resource_id",
		"b.id":          "o.build_id",
		"j.id":          "b.job_id",
		"r.id":          "v.resource_id",
		"v.enabled":     true,
		"b.status":      BuildStatusSucceeded,
		"r.pipeline_id": p.id,
	}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var output algorithm.BuildOutput
		err := rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID, &output.BuildID, &output.JobID)
		if err != nil {
			return nil, err
		}

		output.ResourceVersion.CheckOrder = output.CheckOrder

		db.BuildOutputs = append(db.BuildOutputs, output)
	}

	rows, err = psql.Select("v.id, v.check_order, r.id, i.build_id, i.name, j.id").
		From("build_inputs i, builds b, versioned_resources v, jobs j, resources r").
		Where(sq.Eq{
		"v.id":          "i.versioned_resource_id",
		"b.id":          "i.build_id",
		"j.id":          "b.job_id",
		"r.id":          "v.resource_id",
		"v.enabled":     true,
		"r.pipeline_id": p.id,
	}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var input algorithm.BuildInput
		err := rows.Scan(&input.VersionID, &input.CheckOrder, &input.ResourceID, &input.BuildID, &input.InputName, &input.JobID)
		if err != nil {
			return nil, err
		}

		input.ResourceVersion.CheckOrder = input.CheckOrder

		db.BuildInputs = append(db.BuildInputs, input)
	}

	rows, err = psql.Select("v.id, v.check_order, r.id").
		From("versioned_resources v, resources r").
		Where(sq.Eq{
		"r.id":          "v.resource_id",
		"v.enabled":     true,
		"r.pipeline_id": p.id,
	}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var output algorithm.ResourceVersion
		err := rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID)
		if err != nil {
			return nil, err
		}

		db.ResourceVersions = append(db.ResourceVersions, output)
	}

	rows, err = psql.Select("j.name, j.id").
		From("jobs j").
		Where(sq.Eq{"j.pipeline_id": p.id}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.JobIDs[name] = id
	}

	rows, err = psql.Select("r.name, r.id").
		From("resources r").
		Where(sq.Eq{"r.pipeline_id": p.id}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.ResourceIDs[name] = id
	}

	return db, nil
}

func (p *pipeline) saveOutput(buildID int, vr VersionedResource, explicit bool) error {
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var resourceID int
	err = psql.Select("id").
		From("resources").
		Where(sq.Eq{
		"name":        vr.Resource,
		"pipeline_id": p.id,
	}).RunWith(tx).QueryRow().Scan(&resourceID)

	svr, created, err := p.saveVersionedResource(tx, resourceID, vr)
	if err != nil {
		return err
	}

	if created {
		versionJSON, err := json.Marshal(vr.Version)
		if err != nil {
			return err
		}

		err = p.incrementCheckOrderWhenNewerVersion(tx, resourceID, vr.Type, string(versionJSON))
		if err != nil {
			return err
		}
	}

	_, err = psql.Insert("build_outputs").
		Columns("build_id", "versioned_resource_id", "explicit").
		Values(buildID, svr.ID, explicit).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (p *pipeline) saveInputTx(tx Tx, buildID int, input BuildInput) error {
	var resourceID int
	err := psql.Select("id").
		From("resources").
		Where(sq.Eq{
		"name":        input.VersionedResource.Resource,
		"pipeline_id": p.id,
	}).RunWith(tx).QueryRow().Scan(&resourceID)

	svr, _, err := p.saveVersionedResource(tx, resourceID, input.VersionedResource)
	if err != nil {
		return err
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
		return err
	}

	return nil
}

func (p *pipeline) saveVersionedResource(tx Tx, resourceID int, vr VersionedResource) (SavedVersionedResource, bool, error) {
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
	`, resourceID, vr.Type, string(versionJSON), string(metadataJSON))

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
		err = psql.Update("versioned_resources").
			Set("metadata", string(metadataJSON)).
			Set("modified_time", sq.Expr("now()")).
			Where(sq.Eq{
			"resource_id": resourceID,
			"type":        vr.Type,
			"version":     string(versionJSON),
		}).
			Suffix("RETURNING id, enabled, metadata, modified_time, check_order").
			RunWith(tx).
			QueryRow().
			Scan(&id, &enabled, &savedMetadata, &modified_time, &check_order)
	} else {
		err = psql.Select("id, enabled, metadata, modified_time, check_order").
			From("versioned_resources").
			Where(sq.Eq{
			"resource_id": resourceID,
			"type":        vr.Type,
			"version":     string(versionJSON),
		}).
			RunWith(tx).
			QueryRow().
			Scan(&id, &enabled, &savedMetadata, &modified_time, &check_order)
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

func (p *pipeline) incrementCheckOrderWhenNewerVersion(tx Tx, resourceID int, resourceType string, version string) error {
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

func (p *pipeline) getJobBuildInputs(table string, jobName string) ([]BuildInput, error) {
	rows, err := psql.Select("i.input_name, i.first_occurrence, r.name, v.type, v.version, v.metadata").
		From(table + " i").
		Join("jobs j ON i.job_id = j.id").
		Join("versioned_resources v ON v.id = i.versioned_id").
		Join("resources r ON r.id = v.resource_id").
		Where(sq.Eq{
		"j.name":        jobName,
		"j.pipeline_id": p.id,
	}).
		RunWith(p.conn).
		Query()
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
			version         ResourceVersion
			metadata        []ResourceMetadataField
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
				Resource: resourceName,
				Type:     resourceType,
				Version:  version,
				Metadata: metadata,
			},
			FirstOccurrence: firstOccurrence,
		})
	}
	return buildInputs, nil
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
