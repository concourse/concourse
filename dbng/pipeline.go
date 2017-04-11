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
	CreateResource(name string, config atc.ResourceConfig) (*Resource, error)

	AcquireResourceCheckingLockWithIntervalCheck(
		logger lager.Logger,
		resource *Resource,
		resourceTypes atc.VersionedResourceTypes,
		length time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	LoadVersionsDB() (*algorithm.VersionsDB, error)

	Resource(name string) (Resource, bool, error)

	ResourceTypes() ([]ResourceType, error)
	ResourceType(name string) (ResourceType, bool, error)

	Job(name string) (Job, bool, error)

	Destroy() error
	Expose() error
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

	err = createBuildEventSeq(tx, buildID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &build{
		id:         buildID,
		pipelineID: p.id,
		teamID:     p.teamID,
		conn:       p.conn,
	}, nil
}

func (p *pipeline) CreateResource(name string, config atc.ResourceConfig) (*Resource, error) {
	configPayload, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var resourceID int
	err = psql.Insert("resources").
		Columns("pipeline_id", "name", "config", "source_hash").
		Values(p.id, name, configPayload, mapHash(config.Source)).
		Suffix("RETURNING id").
		RunWith(p.conn).
		QueryRow().
		Scan(&resourceID)
	if err != nil {
		return nil, err
	}

	resource := &resource{conn: p.conn}
	err = scanResource(resource, resourcesQuery.
		Where(sq.Eq{"id": resourceID}).
		RunWith(p.conn).
		QueryRow(),
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return resource, nil
}

func (p *pipeline) Resource(name string) (*Resource, error) {
	row := resourcesQuery.Where(sq.Eq{
		"pipeline_id": p.id,
		"name":        name,
		"active":      true,
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
