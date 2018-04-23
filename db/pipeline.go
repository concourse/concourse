package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
)

type ErrResourceNotFound struct {
	Name string
}

func (e ErrResourceNotFound) Error() string {
	return fmt.Sprintf("resource '%s' not found", e.Name)
}

//go:generate counterfeiter . Pipeline

type Cause struct {
	VersionedResourceID int `json:"versioned_resource_id"`
	BuildID             int `json:"build_id"`
}

type Pipeline interface {
	ID() int
	Name() string
	TeamID() int
	TeamName() string
	Groups() atc.GroupConfigs
	ConfigVersion() ConfigVersion
	Public() bool
	Paused() bool
	ScopedName(string) string

	CheckPaused() (bool, error)
	Reload() (bool, error)

	Causality(versionedResourceID int) ([]Cause, error)

	SetResourceCheckError(Resource, error) error
	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error)

	GetAllPendingBuilds() (map[string][]Build, error)

	GetLatestVersionedResource(resourceName string) (SavedVersionedResource, bool, error)
	GetVersionedResourceByVersion(atcVersion atc.Version, resourceName string) (SavedVersionedResource, bool, error)

	VersionedResource(versionedResourceID int) (SavedVersionedResource, bool, error)
	DisableVersionedResource(versionedResourceID int) error
	EnableVersionedResource(versionedResourceID int) error
	GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error)
	GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error)
	Builds(page Page) ([]Build, Pagination, error)

	DeleteBuildEventsByBuildIDs(buildIDs []int) error

	AcquireSchedulingLock(lager.Logger, time.Duration) (lock.Lock, bool, error)

	AcquireResourceCheckingLockWithIntervalCheck(
		logger lager.Logger,
		resourceName string,
		usedResourceConfig *UsedResourceConfig,
		interval time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	AcquireResourceTypeCheckingLockWithIntervalCheck(
		logger lager.Logger,
		resourceTypeName string,
		usedResourceConfig *UsedResourceConfig,
		interval time.Duration,
		immediate bool,
	) (lock.Lock, bool, error)

	LoadVersionsDB() (*algorithm.VersionsDB, error)

	Resource(name string) (Resource, bool, error)
	Resources() (Resources, error)

	ResourceTypes() (ResourceTypes, error)
	ResourceType(name string) (ResourceType, bool, error)

	Job(name string) (Job, bool, error)
	Jobs() (Jobs, error)
	Dashboard() (Dashboard, error)

	Expose() error
	Hide() error

	Pause() error
	Unpause() error

	Destroy() error
	Rename(string) error

	CreateOneOffBuild() (Build, error)
}

type pipeline struct {
	id            int
	name          string
	teamID        int
	teamName      string
	groups        atc.GroupConfigs
	configVersion ConfigVersion
	paused        bool
	public        bool

	cachedAt   time.Time
	versionsDB *algorithm.VersionsDB

	conn        Conn
	lockFactory lock.LockFactory
}

//ConfigVersion is a sequence identifier used for compare-and-swap
type ConfigVersion int

type PipelinePausedState string

var pipelinesQuery = psql.Select(`
		p.id,
		p.name,
		p.groups,
		p.version,
		p.team_id,
		t.name,
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
func (p *pipeline) Groups() atc.GroupConfigs     { return p.groups }
func (p *pipeline) ConfigVersion() ConfigVersion { return p.configVersion }
func (p *pipeline) Public() bool                 { return p.public }
func (p *pipeline) Paused() bool                 { return p.paused }

func (p *pipeline) ScopedName(n string) string {
	return p.name + ":" + n
}

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
			VersionedResourceID: vrID,
			BuildID:             buildID,
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

	build := &build{conn: p.conn, lockFactory: p.lockFactory}
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

func (p *pipeline) SetResourceCheckError(resource Resource, cause error) error {
	var err error

	if cause == nil {
		_, err = psql.Update("resources").
			Set("check_error", nil).
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

func (p *pipeline) GetAllPendingBuilds() (map[string][]Build, error) {
	builds := map[string][]Build{}

	rows, err := buildsQuery.
		Where(sq.Eq{
			"b.status":      BuildStatusPending,
			"j.active":      true,
			"b.pipeline_id": p.id,
		}).
		OrderBy("b.id").
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		build := &build{conn: p.conn, lockFactory: p.lockFactory}
		err = scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		builds[build.JobName()] = append(builds[build.JobName()], build)
	}

	return builds, nil
}

func (p *pipeline) SaveResourceVersions(config atc.ResourceConfig, versions []atc.Version) error {
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	for _, version := range versions {
		vr := VersionedResource{
			Resource: config.Name,
			Type:     config.Type,
			Version:  ResourceVersion(version),
		}

		versionJSON, err := json.Marshal(vr.Version)
		if err != nil {
			return err
		}

		var resourceID int
		err = psql.Select("id").
			From("resources").
			Where(sq.Eq{
				"name":        vr.Resource,
				"pipeline_id": p.id,
			}).RunWith(tx).QueryRow().Scan(&resourceID)
		if err != nil {
			return err
		}

		_, _, err = p.saveVersionedResource(tx, resourceID, vr)
		if err != nil {
			return err
		}

		err = p.incrementCheckOrderWhenNewerVersion(tx, resourceID, vr.Type, string(versionJSON))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *pipeline) GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error) {
	var resourceID int
	err := psql.Select("id").
		From("resources").
		Where(sq.Eq{
			"name":        resourceName,
			"pipeline_id": p.id,
			"active":      true,
		}).RunWith(p.conn).QueryRow().Scan(&resourceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []SavedVersionedResource{}, Pagination{}, false, nil
		}

		return []SavedVersionedResource{}, Pagination{}, false, err
	}

	query := `
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, r.name, v.check_order
		FROM versioned_resources v
		INNER JOIN resources r ON v.resource_id = r.id
		WHERE v.resource_id = $1
	`

	var rows *sql.Rows
	if page.Until != 0 {
		rows, err = p.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND v.check_order > (SELECT check_order FROM versioned_resources WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), resourceID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.Since != 0 {
		rows, err = p.conn.Query(fmt.Sprintf(`
			%s
				AND v.check_order < (SELECT check_order FROM versioned_resources WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), resourceID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.To != 0 {
		rows, err = p.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND v.check_order >= (SELECT check_order FROM versioned_resources WHERE id = $2)
				ORDER BY v.check_order ASC
				LIMIT $3
			) sub
			ORDER BY sub.check_order DESC
		`, query), resourceID, page.To, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.From != 0 {
		rows, err = p.conn.Query(fmt.Sprintf(`
			%s
				AND v.check_order <= (SELECT check_order FROM versioned_resources WHERE id = $2)
			ORDER BY v.check_order DESC
			LIMIT $3
		`, query), resourceID, page.From, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else {
		rows, err = p.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY v.check_order DESC
			LIMIT $2
		`, query), resourceID, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	}

	defer Close(rows)

	savedVersionedResources := make([]SavedVersionedResource, 0)
	for rows.Next() {
		var savedVersionedResource SavedVersionedResource

		var versionString, metadataString string

		err = rows.Scan(
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

		savedVersionedResources = append(savedVersionedResources, savedVersionedResource)
	}

	if len(savedVersionedResources) == 0 {
		return []SavedVersionedResource{}, Pagination{}, true, nil
	}

	var minCheckOrder int
	var maxCheckOrder int

	err = p.conn.QueryRow(`
		SELECT COALESCE(MAX(v.check_order), 0) as maxCheckOrder,
			COALESCE(MIN(v.check_order), 0) as minCheckOrder
		FROM versioned_resources v
		WHERE v.resource_id = $1
	`, resourceID).Scan(&maxCheckOrder, &minCheckOrder)
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

func (p *pipeline) GetLatestVersionedResource(resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource: resourceName,
		},
	}

	err := psql.Select("v.id, v.enabled, v.type, v.version, v.metadata, v.modified_time, v.check_order").
		From("versioned_resources v, resources r").
		Where(sq.Eq{
			"r.name":        resourceName,
			"r.pipeline_id": p.id,
		}).
		Where(sq.Expr("v.resource_id = r.id")).
		OrderBy("check_order DESC").
		Limit(1).
		RunWith(p.conn).
		QueryRow().
		Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.ModifiedTime, &svr.CheckOrder)
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

func (p *pipeline) GetVersionedResourceByVersion(atcVersion atc.Version, resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	versionJSON, err := json.Marshal(atcVersion)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource: resourceName,
		},
	}

	err = psql.Select("v.id", "v.enabled", "v.type", "v.version", "v.metadata", "v.check_order").
		From("versioned_resources v").
		Join("resources r ON r.id = v.resource_id").
		Where(sq.Eq{
			"v.version":     string(versionJSON),
			"r.name":        resourceName,
			"r.pipeline_id": p.id,
			"enabled":       true,
		}).
		RunWith(p.conn).
		QueryRow().
		Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.CheckOrder)
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

func (p *pipeline) VersionedResource(versionedResourceID int) (SavedVersionedResource, bool, error) {
	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{},
	}

	var versionBytes, metadataBytes string
	err := psql.Select("v.id", "v.enabled", "v.type", "v.version", "v.metadata", "v.check_order", "r.name").
		From("versioned_resources v").
		Join("resources r ON r.id = v.resource_id").
		Where(sq.Eq{"v.id": versionedResourceID}).
		RunWith(p.conn).
		QueryRow().
		Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.CheckOrder, &svr.VersionedResource.Resource)
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

func (p *pipeline) DisableVersionedResource(versionedResourceID int) error {
	return p.toggleVersionedResource(versionedResourceID, false)
}

func (p *pipeline) EnableVersionedResource(versionedResourceID int) error {
	return p.toggleVersionedResource(versionedResourceID, true)
}

func (p *pipeline) GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error) {
	rows, err := buildsQuery.
		JoinClause("LEFT OUTER JOIN build_inputs bi ON bi.build_id = b.id").
		Where(sq.Eq{
			"bi.versioned_resource_id": versionedResourceID,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	builds := []Build{}
	for rows.Next() {
		build := &build{conn: p.conn, lockFactory: p.lockFactory}
		err = scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}
		builds = append(builds, build)
	}

	return builds, err
}

func (p *pipeline) GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error) {
	rows, err := buildsQuery.
		JoinClause("LEFT OUTER JOIN build_outputs bo ON bo.build_id = b.id").
		Where(sq.Eq{
			"bo.versioned_resource_id": versionedResourceID,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	builds := []Build{}
	for rows.Next() {
		build := &build{conn: p.conn, lockFactory: p.lockFactory}
		err = scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, err
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

func (p *pipeline) Builds(page Page) ([]Build, Pagination, error) {
	return getBuildsWithPagination(buildsQuery.Where(sq.Eq{"b.pipeline_id": p.id}), page, p.conn, p.lockFactory)
}

func (p *pipeline) Resources() (Resources, error) {
	rows, err := resourcesQuery.Where(sq.Eq{"r.pipeline_id": p.id}).RunWith(p.conn).Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

	var resources Resources

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

func (p *pipeline) ResourceTypes() (ResourceTypes, error) {
	rows, err := resourceTypesQuery.Where(sq.Eq{"pipeline_id": p.id}).RunWith(p.conn).Query()
	if err != nil {
		return nil, err
	}
	defer Close(rows)

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
	row := jobsQuery.Where(sq.Eq{
		"j.name":        name,
		"j.active":      true,
		"j.pipeline_id": p.id,
	}).RunWith(p.conn).QueryRow()

	job := &job{conn: p.conn, lockFactory: p.lockFactory}
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
	if err != nil {
		return nil, err
	}

	nextBuilds, err := p.getBuildsFrom("next_builds_per_job")
	if err != nil {
		return nil, err
	}

	finishedBuilds, err := p.getBuildsFrom("latest_completed_builds_per_job")
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

	defer Rollback(tx)

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
	latestModifiedTime, err := p.getLatestModifiedTime()
	if err != nil {
		return nil, err
	}

	if p.versionsDB != nil && p.cachedAt.Equal(latestModifiedTime) {
		return p.versionsDB, nil
	}

	db := &algorithm.VersionsDB{
		BuildOutputs:     []algorithm.BuildOutput{},
		BuildInputs:      []algorithm.BuildInput{},
		ResourceVersions: []algorithm.ResourceVersion{},
		JobIDs:           map[string]int{},
		ResourceIDs:      map[string]int{},
	}

	rows, err := psql.Select("v.id, v.check_order, r.id, o.build_id, b.job_id").
		From("build_outputs o, builds b, versioned_resources v, resources r").
		Where(sq.Expr("v.id = o.versioned_resource_id")).
		Where(sq.Expr("b.id = o.build_id")).
		Where(sq.Expr("r.id = v.resource_id")).
		Where(sq.Eq{
			"v.enabled":     true,
			"b.status":      BuildStatusSucceeded,
			"r.pipeline_id": p.id,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var output algorithm.BuildOutput
		err = rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID, &output.BuildID, &output.JobID)
		if err != nil {
			return nil, err
		}

		output.ResourceVersion.CheckOrder = output.CheckOrder

		db.BuildOutputs = append(db.BuildOutputs, output)
	}

	rows, err = psql.Select("v.id, v.check_order, r.id, i.build_id, i.name, b.job_id, b.status = 'succeeded'").
		From("build_inputs i, builds b, versioned_resources v, resources r").
		Where(sq.Expr("v.id = i.versioned_resource_id")).
		Where(sq.Expr("b.id = i.build_id")).
		Where(sq.Expr("r.id = v.resource_id")).
		Where(sq.Eq{
			"v.enabled":     true,
			"r.pipeline_id": p.id,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var succeeded bool

		var input algorithm.BuildInput
		err = rows.Scan(&input.VersionID, &input.CheckOrder, &input.ResourceID, &input.BuildID, &input.InputName, &input.JobID, &succeeded)
		if err != nil {
			return nil, err
		}

		input.ResourceVersion.CheckOrder = input.CheckOrder

		db.BuildInputs = append(db.BuildInputs, input)

		if succeeded {
			// implicit output
			db.BuildOutputs = append(db.BuildOutputs, algorithm.BuildOutput{
				ResourceVersion: input.ResourceVersion,
				JobID:           input.JobID,
				BuildID:         input.BuildID,
			})
		}
	}

	rows, err = psql.Select("v.id, v.check_order, r.id").
		From("versioned_resources v, resources r").
		Where(sq.Expr("r.id = v.resource_id")).
		Where(sq.Eq{
			"v.enabled":     true,
			"r.pipeline_id": p.id,
		}).
		RunWith(p.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var output algorithm.ResourceVersion
		err = rows.Scan(&output.VersionID, &output.CheckOrder, &output.ResourceID)
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
		RunWith(p.conn).
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

	p.versionsDB = db
	p.cachedAt = latestModifiedTime

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

func (p *pipeline) AcquireSchedulingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	lock, acquired, err := p.lockFactory.Acquire(
		logger.Session("lock", lager.Data{
			"pipeline": p.name,
		}),
		lock.NewPipelineSchedulingLockLockID(p.id),
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	var keepLock bool
	defer func() {
		if !keepLock {
			err = lock.Release()
			if err != nil {
				logger.Error("failed-to-release-lock", err)
			}
		}
	}()

	result, err := p.conn.Exec(`
		UPDATE pipelines
		SET last_scheduled = now()
		WHERE id = $1
			AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
	`, p.id, interval.Seconds())
	if err != nil {
		return nil, false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return nil, false, err
	}

	if rows == 0 {
		return nil, false, nil
	}

	keepLock = true

	return lock, true, nil
}

func (p *pipeline) saveOutput(buildID int, vr VersionedResource) error {
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	var resourceID int
	err = psql.Select("id").
		From("resources").
		Where(sq.Eq{
			"name":        vr.Resource,
			"pipeline_id": p.id,
		}).RunWith(tx).QueryRow().Scan(&resourceID)
	if err != nil {
		if err == sql.ErrNoRows {
			return ErrResourceNotFound{Name: vr.Resource}
		}
		return err
	}

	svr, created, err := p.saveVersionedResource(tx, resourceID, vr)
	if err != nil {
		return err
	}

	if created {
		var versionJSON []byte
		versionJSON, err = json.Marshal(vr.Version)
		if err != nil {
			return err
		}

		err = p.incrementCheckOrderWhenNewerVersion(tx, resourceID, vr.Type, string(versionJSON))
		if err != nil {
			return err
		}
	}

	_, err = psql.Insert("build_outputs").
		Columns("build_id", "versioned_resource_id").
		Values(buildID, svr.ID).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (p *pipeline) CreateOneOffBuild() (Build, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	build := &build{conn: p.conn, lockFactory: p.lockFactory}
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

func (p *pipeline) saveInputTx(tx Tx, buildID int, input BuildInput) error {
	var resourceID int
	err := psql.Select("id").
		From("resources").
		Where(sq.Eq{
			"name":        input.VersionedResource.Resource,
			"pipeline_id": p.id,
		}).RunWith(tx).QueryRow().Scan(&resourceID)
	if err != nil {
		return err
	}

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

	return swallowUniqueViolation(err)
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
	var modifiedTime time.Time
	var checkOrder int

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
			Scan(&id, &enabled, &savedMetadata, &modifiedTime, &checkOrder)
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
			Scan(&id, &enabled, &savedMetadata, &modifiedTime, &checkOrder)
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
		ModifiedTime: modifiedTime,

		VersionedResource: vr,
		CheckOrder:        checkOrder,
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
	return err
}

func (p *pipeline) toggleVersionedResource(versionedResourceID int, enable bool) error {
	rows, err := psql.Update("versioned_resources").
		Set("enabled", enable).
		Set("modified_time", sq.Expr("now()")).
		Where(sq.Eq{"id": versionedResourceID}).
		RunWith(p.conn).
		Exec()
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

func (p *pipeline) getLatestModifiedTime() (time.Time, error) {
	var maxModifiedTime time.Time

	err := p.conn.QueryRow(`
	SELECT
		CASE
			WHEN b_max > vr_max AND b_max > bi_max THEN b_max
			WHEN bi_max > vr_max THEN bi_max
			ELSE vr_max
		END
	FROM
		(
			SELECT COALESCE(MAX(b.end_time), 'epoch') as b_max
			FROM builds b
			WHERE b.pipeline_id = $1
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
	`, p.id).Scan(&maxModifiedTime)

	return maxModifiedTime, err
}

func (p *pipeline) getBuildsFrom(view string) (map[string]Build, error) {
	rows, err := buildsQuery.
		From(view + " b").
		Where(sq.Eq{"b.pipeline_id": p.id}).
		RunWith(p.conn).Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	nextBuilds := make(map[string]Build)

	for rows.Next() {
		build := &build{conn: p.conn, lockFactory: p.lockFactory}
		err := scanBuild(build, rows, p.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}
		nextBuilds[build.JobName()] = build
	}

	return nextBuilds, nil
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
