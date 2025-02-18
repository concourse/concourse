package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/util"
)

type ResourceTypeNotFoundError struct {
	ID int
}

func (e ResourceTypeNotFoundError) Error() string {
	return fmt.Sprintf("resource type not found: %d", e.ID)
}

//counterfeiter:generate . ResourceType
type ResourceType interface {
	PipelineRef

	ID() int
	TeamID() int
	TeamName() string
	Name() string
	Type() string
	Privileged() bool
	Source() atc.Source
	Defaults() atc.Source
	Params() atc.Params
	Tags() atc.Tags
	CheckEvery() *atc.CheckEvery
	CheckTimeout() string
	LastCheckStartTime() time.Time
	LastCheckEndTime() time.Time
	CurrentPinnedVersion() atc.Version
	ResourceConfigID() int
	ResourceConfigScopeID() int

	HasWebhook() bool

	SetResourceConfigScope(ResourceConfigScope) error
	SharedResourcesAndTypes() (atc.ResourcesAndTypes, error)

	CheckPlan(planFactory atc.PlanFactory, imagePlanner atc.ImagePlanner, from atc.Version, interval atc.CheckEvery, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) atc.Plan
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)
	CreateInMemoryBuild(context.Context, atc.Plan, util.SequenceGenerator) (Build, error)

	ClearVersions() (int64, error)

	Reload() (bool, error)
}

type ResourceTypes []ResourceType

func (resourceTypes ResourceTypes) Parent(checkable Checkable) (ResourceType, bool) {
	for _, t := range resourceTypes {
		if t.PipelineID() == checkable.PipelineID() {
			if t != checkable && t.Name() == checkable.Type() {
				return t, true
			}
		}
	}
	return nil, false
}

func (resourceTypes ResourceTypes) Filter(checkable Checkable) ResourceTypes {
	var result ResourceTypes

	for {
		resourceType, found := resourceTypes.Parent(checkable)
		if !found {
			return result
		}

		result = append(result, resourceType)
		checkable = resourceType
	}
}

func (resourceTypes ResourceTypes) Deserialize() atc.ResourceTypes {
	var atcResourceTypes atc.ResourceTypes

	for _, t := range resourceTypes {
		// Apply source defaults to resource types
		source := t.Source()
		parentType, found := resourceTypes.Parent(t)
		if found {
			source = parentType.Defaults().Merge(source)
		} else {
			defaults, found := atc.FindBaseResourceTypeDefaults(t.Type())
			if found {
				source = defaults.Merge(source)
			}
		}

		atcResourceTypes = append(atcResourceTypes, atc.ResourceType{
			Name:       t.Name(),
			Type:       t.Type(),
			Source:     source,
			Defaults:   t.Defaults(),
			Privileged: t.Privileged(),
			CheckEvery: t.CheckEvery(),
			Tags:       t.Tags(),
			Params:     t.Params(),
		})
	}

	return atcResourceTypes
}

func (resourceTypes ResourceTypes) Configs() atc.ResourceTypes {
	var configs atc.ResourceTypes

	for _, r := range resourceTypes {
		configs = append(configs, atc.ResourceType{
			Name:       r.Name(),
			Type:       r.Type(),
			Source:     r.Source(),
			Defaults:   r.Defaults(),
			Privileged: r.Privileged(),
			CheckEvery: r.CheckEvery(),
			Tags:       r.Tags(),
			Params:     r.Params(),
		})
	}

	return configs
}

func (resourceTypes ResourceTypes) Without(name string) ResourceTypes {
	newTypes := ResourceTypes{}
	for _, t := range resourceTypes {
		if t.Name() != name {
			newTypes = append(newTypes, t)
		}
	}

	return newTypes
}

var resourceTypesQuery = psql.Select(
	"r.id",
	"r.pipeline_id",
	"r.name",
	"r.type",
	"r.config",
	"r.nonce",
	"p.name",
	"p.instance_vars",
	"t.id",
	"t.name",
	"r.resource_config_id",
	"ro.id",
	"ro.last_check_start_time",
	"ro.last_check_end_time",
).
	From("resource_types r").
	Join("pipelines p ON p.id = r.pipeline_id").
	Join("teams t ON t.id = p.team_id").
	LeftJoin("resource_configs c ON c.id = r.resource_config_id").
	LeftJoin("resource_config_scopes ro ON ro.resource_config_id = c.id").
	Where(sq.Eq{"r.active": true})

type resourceType struct {
	pipelineRef

	id                    int
	teamID                int
	resourceConfigID      int
	resourceConfigScopeID int
	teamName              string
	name                  string
	type_                 string
	privileged            bool
	source                atc.Source
	defaults              atc.Source
	params                atc.Params
	tags                  atc.Tags
	checkEvery            *atc.CheckEvery
	lastCheckStartTime    time.Time
	lastCheckEndTime      time.Time
}

func (t *resourceType) ID() int                           { return t.id }
func (t *resourceType) TeamID() int                       { return t.teamID }
func (t *resourceType) TeamName() string                  { return t.teamName }
func (t *resourceType) Name() string                      { return t.name }
func (t *resourceType) Type() string                      { return t.type_ }
func (t *resourceType) Privileged() bool                  { return t.privileged }
func (t *resourceType) CheckEvery() *atc.CheckEvery       { return t.checkEvery }
func (t *resourceType) CheckTimeout() string              { return "" }
func (r *resourceType) LastCheckStartTime() time.Time     { return r.lastCheckStartTime }
func (r *resourceType) LastCheckEndTime() time.Time       { return r.lastCheckEndTime }
func (t *resourceType) Source() atc.Source                { return t.source }
func (t *resourceType) Defaults() atc.Source              { return t.defaults }
func (t *resourceType) Params() atc.Params                { return t.params }
func (t *resourceType) Tags() atc.Tags                    { return t.tags }
func (t *resourceType) ResourceConfigID() int             { return t.resourceConfigID }
func (t *resourceType) ResourceConfigScopeID() int        { return t.resourceConfigScopeID }
func (t *resourceType) CurrentPinnedVersion() atc.Version { return nil }
func (t *resourceType) HasWebhook() bool                  { return false }

func newEmptyResourceType(conn DbConn, lockFactory lock.LockFactory) *resourceType {
	return &resourceType{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
}

func (t *resourceType) Reload() (bool, error) {
	row := resourceTypesQuery.Where(sq.Eq{"r.id": t.id}).RunWith(t.conn).QueryRow()

	err := scanResourceType(t, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *resourceType) SetResourceConfigScope(scope ResourceConfigScope) error {
	return setResourceConfigScopeForResourceType(r.conn, scope, sq.Eq{"id": r.id})
}

func setResourceConfigScopeForResourceType(conn sq.Runner, scope ResourceConfigScope, pred interface{}, args ...interface{}) error {
	_, err := psql.Update("resource_types").
		Set("resource_config_id", scope.ResourceConfig().ID()).
		Where(pred, args...).
		Where(sq.Or{
			sq.Eq{"resource_config_id": nil},
			sq.NotEq{"resource_config_id": scope.ResourceConfig().ID()},
		}).
		RunWith(conn).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceType) CheckPlan(planFactory atc.PlanFactory, imagePlanner atc.ImagePlanner, from atc.Version, interval atc.CheckEvery, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) atc.Plan {
	plan := planFactory.NewPlan(atc.CheckPlan{
		Name:   r.name,
		Type:   r.type_,
		Source: sourceDefaults.Merge(r.source),
		Tags:   r.tags,

		FromVersion: from,
		Interval:    interval,

		SkipInterval: skipInterval,

		ResourceType: r.name,
	})

	plan.Check.TypeImage = imagePlanner.ImageForType(plan.ID, r.type_, r.tags, skipInterval && skipIntervalRecursively)
	return plan
}

func (r *resourceType) CreateBuild(ctx context.Context, manuallyTriggered bool, plan atc.Plan) (Build, bool, error) {
	tx, err := r.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	if !manuallyTriggered {
		var numRunningBuilds int
		err = psql.Select("COUNT(1)").
			From("builds").
			Where(sq.Eq{"resource_type_id": r.id, "completed": false}).
			RunWith(tx).
			QueryRow().
			Scan(&numRunningBuilds)
		if err != nil {
			return nil, false, err
		}

		if numRunningBuilds > 0 {
			// a build is already running; leave it be
			return nil, false, nil
		}
	}

	build := newEmptyBuild(r.conn, r.lockFactory)
	err = createStartedBuild(tx, build, startedBuildArgs{
		Name:              CheckBuildName,
		PipelineID:        r.pipelineID,
		TeamID:            r.teamID,
		Plan:              plan,
		ManuallyTriggered: manuallyTriggered,
		SpanContext:       NewSpanContext(ctx),
		ExtraValues: map[string]interface{}{
			"resource_type_id": r.id,
		},
	})
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	err = r.conn.Bus().Notify(atc.ComponentBuildTracker)
	if err != nil {
		return nil, false, err
	}

	_, err = build.Reload()
	if err != nil {
		return nil, false, err
	}

	return build, true, nil
}

func (r *resourceType) CreateInMemoryBuild(ctx context.Context, plan atc.Plan, seqGen util.SequenceGenerator) (Build, error) {
	return nil, errors.New("resource type not supporting in-memory check build as lidar no longer checking resource types")
}

func (r *resourceType) ClearVersions() (int64, error) {
	results, err := psql.Delete("resource_config_versions").
		Where(sq.Eq{
			"resource_config_scope_id": r.resourceConfigScopeID,
		}).
		RunWith(r.conn).
		Exec()

	rowsDeleted, err := results.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rowsDeleted, err
}

func (r *resourceType) SharedResourcesAndTypes() (atc.ResourcesAndTypes, error) {
	return sharedResourcesAndTypes(r.conn, r.resourceConfigScopeID, r.name)
}

func scanResourceType(t *resourceType, row scannable) error {
	var (
		configJSON                           sql.NullString
		rcsID, nonce                         sql.NullString
		lastCheckStartTime, lastCheckEndTime sql.NullTime
		pipelineInstanceVars                 sql.NullString
		resourceConfigID                     sql.NullInt64
	)

	err := row.Scan(&t.id, &t.pipelineID, &t.name, &t.type_, &configJSON,
		&nonce, &t.pipelineName, &pipelineInstanceVars,
		&t.teamID, &t.teamName, &resourceConfigID, &rcsID,
		&lastCheckStartTime, &lastCheckEndTime)
	if err != nil {
		return err
	}

	t.lastCheckStartTime = lastCheckStartTime.Time
	t.lastCheckEndTime = lastCheckEndTime.Time

	es := t.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	var config atc.ResourceType
	if configJSON.Valid {
		decryptedConfig, err := es.Decrypt(configJSON.String, noncense)
		if err != nil {
			return err
		}

		err = json.Unmarshal(decryptedConfig, &config)
		if err != nil {
			return err
		}
	} else {
		config = atc.ResourceType{}
	}

	t.source = config.Source
	t.defaults = config.Defaults
	t.params = config.Params
	t.privileged = config.Privileged
	t.tags = config.Tags
	t.checkEvery = config.CheckEvery

	if resourceConfigID.Valid {
		t.resourceConfigID = int(resourceConfigID.Int64)
	}

	if rcsID.Valid {
		t.resourceConfigScopeID, err = strconv.Atoi(rcsID.String)
		if err != nil {
			return err
		}
	}

	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &t.pipelineInstanceVars)
		if err != nil {
			return err
		}
	}

	return nil
}
