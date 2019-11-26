package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/vars"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
)

type ErrResourceNotFound struct {
	Name string
}

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.Name)
}

//go:generate counterfeiter . Pipeline

type Cause struct {
	ResourceVersionID int `json:"resource_version_id"`
	BuildID           int `json:"build_id"`
}

type Pipeline interface {
	ID() int
	Name() string
	TeamID() int
	TeamName() string
	Groups() atc.GroupConfigs
	VarSources() atc.VarSourceConfigs
	ConfigVersion() ConfigVersion
	Public() bool
	Paused() bool

	CheckPaused() (bool, error)
	Reload() (bool, error)

	Causality(versionedResourceID int) ([]Cause, error)
	ResourceVersion(resourceConfigVersionID int) (atc.ResourceVersion, bool, error)

	GetBuildsWithVersionAsInput(int, int) ([]Build, error)
	GetBuildsWithVersionAsOutput(int, int) ([]Build, error)
	Builds(page Page) ([]Build, Pagination, error)

	CreateOneOffBuild() (Build, error)
	CreateStartedBuild(plan atc.Plan) (Build, error)

	BuildsWithTime(page Page) ([]Build, Pagination, error)

	DeleteBuildEventsByBuildIDs(buildIDs []int) error

	LoadDebugVersionsDB() (*atc.DebugVersionsDB, error)

	Resource(name string) (Resource, bool, error)
	ResourceByID(id int) (Resource, bool, error)
	Resources() (Resources, error)

	ResourceTypes() (ResourceTypes, error)
	ResourceType(name string) (ResourceType, bool, error)
	ResourceTypeByID(id int) (ResourceType, bool, error)

	Job(name string) (Job, bool, error)
	Jobs() (Jobs, error)
	Dashboard() (Dashboard, error)

	Expose() error
	Hide() error

	Pause() error
	Unpause() error

	Destroy() error
	Rename(string) error

	Variables(lager.Logger, creds.Secrets, creds.VarSourcePool) (vars.Variables, error)
}

type pipeline struct {
	id            int
	name          string
	teamID        int
	teamName      string
	groups        atc.GroupConfigs
	varSources    atc.VarSourceConfigs
	configVersion ConfigVersion
	paused        bool
	public        bool

	conn        Conn
	lockFactory lock.LockFactory
}

// ConfigVersion is a sequence identifier used for compare-and-swap.
type ConfigVersion int

var pipelinesQuery = psql.Select(`
		p.id,
		p.name,
		p.groups,
		p.var_sources,
		p.nonce,
		p.version,
		p.team_id,
		t.name,
		p.paused,
		p.public
	`).
	From("pipelines p").
	LeftJoin("teams t ON p.team_id = t.id")

func newPipeline(conn Conn, lockFactory lock.LockFactory) *pipeline {
	return &pipeline{
		conn:        conn,
		lockFactory: lockFactory,
	}
}

func (p *pipeline) ID() int                  { return p.id }
func (p *pipeline) Name() string             { return p.name }
func (p *pipeline) TeamID() int              { return p.teamID }
func (p *pipeline) TeamName() string         { return p.teamName }
func (p *pipeline) Groups() atc.GroupConfigs { return p.groups }

func (p *pipeline) VarSources() atc.VarSourceConfigs { return p.varSources }
func (p *pipeline) ConfigVersion() ConfigVersion     { return p.configVersion }
func (p *pipeline) Public() bool                     { return p.public }
func (p *pipeline) Paused() bool                     { return p.paused }

// IMPORTANT: This method is broken with the new resource config versions changes
func (p *pipeline) Causality(versionedResourceID int) ([]Cause, error) {
	rows, err := p.conn.Query(`
		WITH RECURSIVE causality(versioned_resource_id, build_id) AS (
				SELECT bi.versioned_resource_id, bi.build_id
				FROM build_inputs bi
				WHERE bi.versioned_resource_id = $1
			UNION
				SELECT bi.versioned_resource_id, bi.build_id
				FROM causality t
				INNER JOIN build_outputs bo ON bo.build_id = t.build_id
				INNER JOIN build_inputs bi ON bi.versioned_resource_id = bo.versioned_resource_id
				INNER JOIN builds b ON b.id = bi.build_id
				AND NOT EXISTS (
					SELECT 1
					FROM build_outputs obo
					INNER JOIN builds ob ON ob.id = obo.build_id
					WHERE obo.build_id < bi.build_id
					AND ob.job_id = b.job_id
					AND obo.versioned_resource_id = bi.versioned_resource_id
				)
		)
		SELECT c.versioned_resource_id, c.build_id
		FROM causality c
		INNER JOIN builds b ON b.id = c.build_id
		ORDER BY b.start_time ASC, c.versioned_resource_id ASC
	`, versionedResourceID)
	if err != nil {
		return nil, err
	}

	var causality []Cause
	for rows.Next() {
		var vrID, buildID int
		err := rows.Scan(&vrID, &buildID)
		if err != nil {
			return nil, err
		}

		causality = append(causality, Cause{
			ResourceVersionID: vrID,
			BuildID:           buildID,
		})
	}

	return causality, nil
}

func (p *pipeline) CheckPaused() (bool, error) {
	var paused bool

	err := psql.Select("paused").
		From("pipelines").
		Where(sq.Eq{"id": p.id}).
		RunWith(p.conn).
		QueryRow().
		Scan(&paused)

	if err != nil {
		return false, err
	}

	return paused, nil
}
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

	defer Rollback(tx)

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

	build := newEmptyBuild(p.conn, p.lockFactory)
	err = scanBuild(build, buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(tx).
		QueryRow(),
		p.conn.EncryptionStrategy(),
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

// ResourceVersion is given a resource config version id and returns the
// resource version struct. This method is used by the API call
// GetResourceVersion to get all the attributes for that version of the
// resource.
func (p *pipeline) ResourceVersion(resourceConfigVersionID int) (atc.ResourceVersion, bool, error) {
	rv := atc.ResourceVersion{}
	var (
		versionBytes  string
		metadataBytes string
	)

	enabled := `
		NOT EXISTS (
			SELECT 1
			FROM resource_disabled_versions d, resources r
			WHERE v.version_md5 = d.version_md5
			AND r.resource_config_scope_id = v.resource_config_scope_id
			AND r.id = d.resource_id
		)`

	err := psql.Select("v.id", "v.version", "v.metadata", enabled).
		From("resource_config_versions v").
		Where(sq.Eq{
			"v.id": resourceConfigVersionID,
		}).
		RunWith(p.conn).
		QueryRow().
		Scan(&rv.ID, &versionBytes, &metadataBytes, &rv.Enabled)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.ResourceVersion{}, false, nil
		}

		return atc.ResourceVersion{}, false, err
	}

	err = json.Unmarshal([]byte(versionBytes), &rv.Version)
	if err != nil {
		return atc.ResourceVersion{}, false, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &rv.Metadata)
	if err != nil {
		return atc.ResourceVersion{}, false, err
	}

	return rv, true, nil
}

func (p *pipeline) GetBuildsWithVersionAsInput(resourceID, resourceConfigVersionID int) ([]Build, error) {
	rows, err := buildsQuery.
		Join("build_resource_config_version_inputs bi ON bi.build_id = b.id").
		Join("resource_config_versions rcv ON rcv.version_md5 = bi.version_md5").
		Where(sq.Eq{
			"rcv.id":         resourceConfigVersionID,
			"bi.resource_id": resourceID,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	builds := []Build{}
	for rows.Next() {
		build := newEmptyBuild(p.conn, p.lockFactory)
		err = scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}
		builds = append(builds, build)
	}

	return builds, err
}

func (p *pipeline) GetBuildsWithVersionAsOutput(resourceID, resourceConfigVersionID int) ([]Build, error) {
	rows, err := buildsQuery.
		Join("build_resource_config_version_outputs bo ON bo.build_id = b.id").
		Join("resource_config_versions rcv ON rcv.version_md5 = bo.version_md5").
		Where(sq.Eq{
			"rcv.id":         resourceConfigVersionID,
			"bo.resource_id": resourceID,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	builds := []Build{}
	for rows.Next() {
		build := newEmptyBuild(p.conn, p.lockFactory)
		err = scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, err
}

func (p *pipeline) Resource(name string) (Resource, bool, error) {
	return p.resource(sq.Eq{
		"r.pipeline_id": p.id,
		"r.name":        name,
	})
}

func (p *pipeline) ResourceByID(id int) (Resource, bool, error) {
	return p.resource(sq.Eq{
		"r.pipeline_id": p.id,
		"r.id":          id,
	})
}

func (p *pipeline) resource(where map[string]interface{}) (Resource, bool, error) {
	row := resourcesQuery.
		Where(where).
		RunWith(p.conn).
		QueryRow()

	resource := newEmptyResource(p.conn, p.lockFactory)
	err := scanResource(resource, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return resource, true, nil
}

func (p *pipeline) Builds(page Page) ([]Build, Pagination, error) {
	return getBuildsWithPagination(
		buildsQuery.Where(sq.Eq{"b.pipeline_id": p.id}), minMaxIdQuery, page, p.conn, p.lockFactory)
}

func (p *pipeline) BuildsWithTime(page Page) ([]Build, Pagination, error) {
	return getBuildsWithDates(
		buildsQuery.Where(sq.Eq{"b.pipeline_id": p.id}), minMaxIdQuery, page, p.conn, p.lockFactory)
}

func (p *pipeline) Resources() (Resources, error) {
	return resources(p.id, p.conn, p.lockFactory)
}

func (p *pipeline) ResourceTypes() (ResourceTypes, error) {
	rows, err := resourceTypesQuery.
		Where(sq.Eq{"r.pipeline_id": p.id}).
		OrderBy("r.name").
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	resourceTypes := []ResourceType{}

	for rows.Next() {
		resourceType := newEmptyResourceType(p.conn, p.lockFactory)
		err := scanResourceType(resourceType, rows)
		if err != nil {
			return nil, err
		}

		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (p *pipeline) ResourceType(name string) (ResourceType, bool, error) {
	return p.resourceType(sq.Eq{
		"r.pipeline_id": p.id,
		"r.name":        name,
	})
}

func (p *pipeline) ResourceTypeByID(id int) (ResourceType, bool, error) {
	return p.resourceType(sq.Eq{
		"r.pipeline_id": p.id,
		"r.id":          id,
	})
}

func (p *pipeline) resourceType(where map[string]interface{}) (ResourceType, bool, error) {
	row := resourceTypesQuery.
		Where(where).
		RunWith(p.conn).
		QueryRow()

	resourceType := newEmptyResourceType(p.conn, p.lockFactory)
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
	row := jobsQuery.Where(sq.Eq{
		"j.name":        name,
		"j.active":      true,
		"j.pipeline_id": p.id,
	}).RunWith(p.conn).QueryRow()

	job := newEmptyJob(p.conn, p.lockFactory)
	err := scanJob(job, row)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	return job, true, nil
}

func (p *pipeline) Jobs() (Jobs, error) {
	rows, err := jobsQuery.
		Where(sq.Eq{
			"pipeline_id": p.id,
			"active":      true,
		}).
		OrderBy("j.id ASC").
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	jobs, err := scanJobs(p.conn, p.lockFactory, rows)
	return jobs, err
}

func (p *pipeline) Dashboard() (Dashboard, error) {
	dashboard := Dashboard{}

	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	rows, err := jobsQuery.
		Where(sq.Eq{
			"pipeline_id": p.id,
			"active":      true,
		}).
		OrderBy("j.id ASC").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	jobs, err := scanJobs(p.conn, p.lockFactory, rows)
	if err != nil {
		return nil, err
	}

	nextBuilds, err := p.getBuildsFrom(tx, "next_build_id")
	if err != nil {
		return nil, err
	}

	finishedBuilds, err := p.getBuildsFrom(tx, "latest_completed_build_id")
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		dashboardJob := DashboardJob{
			Job: job,
		}

		if nextBuild, found := nextBuilds[job.Name()]; found {
			dashboardJob.NextBuild = nextBuild
		}

		if finishedBuild, found := finishedBuilds[job.Name()]; found {
			dashboardJob.FinishedBuild = finishedBuild
		}

		dashboard = append(dashboard, dashboardJob)
	}

	return dashboard, nil
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
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Update("pipelines").
		Set("paused", false).
		Where(sq.Eq{
			"id": p.id,
		}).
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = requestScheduleForJobsInPipeline(tx, p.id)
	if err != nil {
		return err
	}

	return tx.Commit()
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
	_, err := psql.Delete("pipelines").
		Where(sq.Eq{
			"id": p.id,
		}).
		RunWith(p.conn).
		Exec()

	return err
}

func (p *pipeline) LoadDebugVersionsDB() (*atc.DebugVersionsDB, error) {
	db := &atc.DebugVersionsDB{
		BuildOutputs:     []atc.DebugBuildOutput{},
		BuildInputs:      []atc.DebugBuildInput{},
		ResourceVersions: []atc.DebugResourceVersion{},
		JobIDs:           map[string]int{},
		ResourceIDs:      map[string]int{},
	}

	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	rows, err := psql.Select("v.id, v.check_order, r.id, o.build_id, b.job_id").
		From("build_resource_config_version_outputs o").
		Join("builds b ON b.id = o.build_id").
		Join("resource_config_versions v ON v.version_md5 = o.version_md5").
		Join("resources r ON r.id = o.resource_id").
		Where(sq.Expr("r.resource_config_scope_id = v.resource_config_scope_id")).
		Where(sq.Expr("(r.id, v.version_md5) NOT IN (SELECT resource_id, version_md5 from resource_disabled_versions)")).
		Where(sq.NotEq{
			"v.check_order": 0,
		}).
		Where(sq.Eq{
			"b.status":      BuildStatusSucceeded,
			"r.pipeline_id": p.id,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var output atc.DebugBuildOutput
		err = rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID, &output.BuildID, &output.JobID)
		if err != nil {
			return nil, err
		}

		output.DebugResourceVersion.CheckOrder = output.CheckOrder

		db.BuildOutputs = append(db.BuildOutputs, output)
	}

	rows, err = psql.Select("v.id, v.check_order, r.id, i.build_id, i.name, b.job_id, b.status = 'succeeded'").
		From("build_resource_config_version_inputs i").
		Join("builds b ON b.id = i.build_id").
		Join("resource_config_versions v ON v.version_md5 = i.version_md5").
		Join("resources r ON r.id = i.resource_id").
		Where(sq.Expr("r.resource_config_scope_id = v.resource_config_scope_id")).
		Where(sq.Expr("(r.id, v.version_md5) NOT IN (SELECT resource_id, version_md5 from resource_disabled_versions)")).
		Where(sq.NotEq{
			"v.check_order": 0,
		}).
		Where(sq.Eq{
			"r.pipeline_id": p.id,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var succeeded bool

		var input atc.DebugBuildInput
		err = rows.Scan(&input.VersionID, &input.CheckOrder, &input.ResourceID, &input.BuildID, &input.InputName, &input.JobID, &succeeded)
		if err != nil {
			return nil, err
		}

		input.DebugResourceVersion.CheckOrder = input.CheckOrder

		db.BuildInputs = append(db.BuildInputs, input)

		if succeeded {
			// implicit output
			db.BuildOutputs = append(db.BuildOutputs, atc.DebugBuildOutput{
				DebugResourceVersion: input.DebugResourceVersion,
				JobID:                input.JobID,
				BuildID:              input.BuildID,
			})
		}
	}

	rows, err = psql.Select("v.id, v.check_order, r.id").
		From("resource_config_versions v").
		Join("resources r ON r.resource_config_scope_id = v.resource_config_scope_id").
		LeftJoin("resource_disabled_versions d ON d.resource_id = r.id AND d.version_md5 = v.version_md5").
		Where(sq.NotEq{
			"v.check_order": 0,
		}).
		Where(sq.Eq{
			"r.pipeline_id": p.id,
			"d.resource_id": nil,
			"d.version_md5": nil,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var output atc.DebugResourceVersion
		err = rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID)
		if err != nil {
			return nil, err
		}

		db.ResourceVersions = append(db.ResourceVersions, output)
	}

	rows, err = psql.Select("j.name, j.id").
		From("jobs j").
		Where(sq.Eq{"j.pipeline_id": p.id}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var name string
		var id int
		err = rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.JobIDs[name] = id
	}

	rows, err = psql.Select("r.name, r.id").
		From("resources r").
		Where(sq.Eq{"r.pipeline_id": p.id}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var name string
		var id int
		err = rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.ResourceIDs[name] = id
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (p *pipeline) DeleteBuildEventsByBuildIDs(buildIDs []int) error {
	if len(buildIDs) == 0 {
		return nil
	}

	interfaceBuildIDs := make([]interface{}, len(buildIDs))
	for i, buildID := range buildIDs {
		interfaceBuildIDs[i] = buildID
	}

	indexStrings := make([]string, len(buildIDs))
	for i := range indexStrings {
		indexStrings[i] = "$" + strconv.Itoa(i+1)
	}

	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.Exec(`
   DELETE FROM build_events
	 WHERE build_id IN (`+strings.Join(indexStrings, ",")+`)
	 `, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET reap_time = now()
		WHERE id IN (`+strings.Join(indexStrings, ",")+`)
	`, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (p *pipeline) CreateOneOffBuild() (Build, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	build := newEmptyBuild(p.conn, p.lockFactory)
	err = createBuild(tx, build, map[string]interface{}{
		"name":        sq.Expr("nextval('one_off_name')"),
		"pipeline_id": p.id,
		"team_id":     p.teamID,
		"status":      BuildStatusPending,
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (p *pipeline) CreateStartedBuild(plan atc.Plan) (Build, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	metadata, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}

	encryptedPlan, nonce, err := p.conn.EncryptionStrategy().Encrypt(metadata)
	if err != nil {
		return nil, err
	}

	build := newEmptyBuild(p.conn, p.lockFactory)
	err = createBuild(tx, build, map[string]interface{}{
		"name":         sq.Expr("nextval('one_off_name')"),
		"pipeline_id":  p.id,
		"team_id":      p.teamID,
		"status":       BuildStatusStarted,
		"start_time":   sq.Expr("now()"),
		"schema":       schema,
		"private_plan": encryptedPlan,
		"public_plan":  plan.Public(),
		"nonce":        nonce,
	})
	if err != nil {
		return nil, err
	}

	err = build.saveEvent(tx, event.Status{
		Status: atc.StatusStarted,
		Time:   build.StartTime().Unix(),
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	if err = p.conn.Bus().Notify(buildStartedChannel()); err != nil {
		return nil, err
	}

	if err = p.conn.Bus().Notify(buildEventsChannel(build.id)); err != nil {
		return nil, err
	}

	return build, nil
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
	return err
}

func (p *pipeline) getBuildsFrom(tx Tx, col string) (map[string]Build, error) {
	rows, err := buildsQuery.
		Where(sq.Eq{
			"b.pipeline_id": p.id,
		}).
		Where(sq.Expr("j." + col + " = b.id")).
		RunWith(tx).Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	nextBuilds := make(map[string]Build)

	for rows.Next() {
		build := newEmptyBuild(p.conn, p.lockFactory)
		err := scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}
		nextBuilds[build.JobName()] = build
	}

	return nextBuilds, nil
}

// Variables creates variables for this pipeline. If this pipeline has its own
// var_sources, a vars.MultiVars containing all pipeline specific var_sources
// plug the global variables, otherwise just return the global variables.
func (p *pipeline) Variables(logger lager.Logger, globalSecrets creds.Secrets, varSourcePool creds.VarSourcePool) (vars.Variables, error) {
	globalVars := creds.NewVariables(globalSecrets, p.TeamName(), p.Name(), false)
	varss := []vars.Variables{}
	for _, cm := range p.varSources {
		factory := creds.ManagerFactories()[cm.Type]
		if factory == nil {
			return nil, fmt.Errorf("unknown credential manager type: %s", cm.Type)
		}

		// Interpolate variables in pipeline credential manager's config
		newConfig, err := creds.NewParams(globalVars, atc.Params{"config": cm.Config}).Evaluate()
		if err != nil {
			return nil, err
		}

		config, ok := newConfig["config"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid config format")
		}
		secrets, err := varSourcePool.FindOrCreate(logger, config, factory)
		if err != nil {
			return nil, err
		}
		varss = append(varss, creds.NewVariables(secrets, p.TeamName(), p.Name(), true))
	}

	if len(varss) == 0 {
		return globalVars, nil
	}

	varss = append(varss, globalVars)
	return vars.NewMultiVars(varss), nil
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

func resources(pipelineID int, conn Conn, lockFactory lock.LockFactory) (Resources, error) {
	rows, err := resourcesQuery.
		Where(sq.Eq{"r.pipeline_id": pipelineID}).
		OrderBy("r.name").
		RunWith(conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var resources Resources

	for rows.Next() {
		newResource := newEmptyResource(conn, lockFactory)
		err := scanResource(newResource, rows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, newResource)
	}

	return resources, nil
}

func requestScheduleForJobsInPipeline(tx Tx, pipelineID int) error {
	_, err := psql.Update("jobs").
		Set("schedule_requested", sq.Expr("now()")).
		Where(sq.Eq{
			"pipeline_id": pipelineID,
		}).
		RunWith(tx).
		Exec()

	return err
}
