package db

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/lagerctx"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
)

//counterfeiter:generate . Checkable
type Checkable interface {
	PipelineRef

	Name() string
	TeamID() int
	ResourceConfigScopeID() int
	TeamName() string
	Type() string
	Source() atc.Source
	Tags() atc.Tags
	CheckEvery() *atc.CheckEvery
	CheckTimeout() string
	LastCheckEndTime() time.Time
	CurrentPinnedVersion() atc.Version

	HasWebhook() bool

	CheckPlan(planFactory atc.PlanFactory, imagePlanner atc.ImagePlanner, from atc.Version, interval atc.CheckEvery, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) atc.Plan
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)
}

//counterfeiter:generate . CheckFactory
type CheckFactory interface {
	TryCreateCheck(context.Context, Checkable, ResourceTypes, atc.Version, bool, bool) (Build, bool, error)
	Resources() ([]Resource, error)
	ResourceTypesByPipeline() (map[int]ResourceTypes, error)
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory

	secrets       creds.Secrets
	varSourcePool creds.VarSourcePool

	planFactory atc.PlanFactory
}

func NewCheckFactory(
	conn Conn,
	lockFactory lock.LockFactory,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,

		secrets:       secrets,
		varSourcePool: varSourcePool,

		planFactory: atc.NewPlanFactory(time.Now().Unix()),
	}
}

func (c *checkFactory) TryCreateCheck(ctx context.Context, checkable Checkable, resourceTypes ResourceTypes, from atc.Version, manuallyTriggered bool, skipIntervalRecursively bool) (Build, bool, error) {
	logger := lagerctx.FromContext(ctx)

	var err error

	sourceDefaults := atc.Source{}
	parentType, found := resourceTypes.Parent(checkable)
	if found {
		sourceDefaults = parentType.Defaults()
	} else {
		defaults, found := atc.FindBaseResourceTypeDefaults(checkable.Type())
		if found {
			sourceDefaults = defaults
		}
	}

	interval := atc.CheckEvery{
		Interval: atc.DefaultCheckInterval,
	}

	if checkable.HasWebhook() {
		interval.Interval = atc.DefaultWebhookInterval
	}

	if checkable.CheckEvery() != nil {
		interval = *checkable.CheckEvery()
	}

	skipInterval := manuallyTriggered
	if !skipInterval && time.Now().Before(checkable.LastCheckEndTime().Add(interval.Interval)) {
		// skip creating the check if its interval hasn't elapsed yet
		return nil, false, nil
	}

	deserializedResourceTypes := resourceTypes.Filter(checkable).Deserialize()
	plan := checkable.CheckPlan(c.planFactory, deserializedResourceTypes, from, interval, sourceDefaults, skipInterval, skipIntervalRecursively)
	build, created, err := checkable.CreateBuild(ctx, manuallyTriggered, plan)
	if err != nil {
		return nil, false, fmt.Errorf("create build: %w", err)
	}

	if !created {
		return nil, false, nil
	}

	logger.Info("created-build", build.LagerData())

	return build, true, nil
}

func (c *checkFactory) Resources() ([]Resource, error) {
	var resources []Resource

	rows, err := resourcesQuery.
		LeftJoin("(select DISTINCT(resource_id) FROM job_inputs) ji ON ji.resource_id = r.id").
		LeftJoin("(select DISTINCT(resource_id) FROM job_outputs) jo ON jo.resource_id = r.id").
		Where(sq.And{
			sq.Eq{"p.paused": false},
		}).
		Where(sq.Or{
			sq.And{
				// find all resources that are inputs to jobs
				sq.NotEq{"ji.resource_id": nil},
			},
			sq.And{
				// find put-only resources that have errored
				sq.Expr("b.status IN ('aborted','failed','errored')"),
				sq.Eq{"ji.resource_id": nil},
			},
		}).
		RunWith(c.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := newEmptyResource(c.conn, c.lockFactory)
		err = scanResource(r, rows)
		if err != nil {
			return nil, err
		}

		resources = append(resources, r)
	}

	return resources, nil
}

func (c *checkFactory) ResourceTypesByPipeline() (map[int]ResourceTypes, error) {
	resourceTypes := make(map[int]ResourceTypes)

	rows, err := resourceTypesQuery.
		Where(sq.And{
			sq.Eq{"p.paused": false},
		}).
		RunWith(c.conn).
		Query()

	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		r := newEmptyResourceType(c.conn, c.lockFactory)
		err = scanResourceType(r, rows)
		if err != nil {
			return nil, err
		}

		resourceTypes[r.pipelineID] = append(resourceTypes[r.pipelineID], r)
	}

	return resourceTypes, nil
}
