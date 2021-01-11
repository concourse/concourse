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

//go:generate counterfeiter . Checkable

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

	CheckPlan(atc.Version, time.Duration, ResourceTypes, atc.Source) atc.CheckPlan
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)
}

//go:generate counterfeiter . CheckFactory

type CheckFactory interface {
	TryCreateCheck(context.Context, Checkable, ResourceTypes, atc.Version, bool) (Build, bool, error)
	Resources() ([]Resource, error)
	ResourceTypes() ([]ResourceType, error)
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory

	secrets       creds.Secrets
	varSourcePool creds.VarSourcePool

	planFactory atc.PlanFactory

	defaultCheckTimeout             time.Duration
	defaultCheckInterval            time.Duration
	defaultWithWebhookCheckInterval time.Duration
}

type CheckDurations struct {
	Timeout             time.Duration
	Interval            time.Duration
	IntervalWithWebhook time.Duration
}

func NewCheckFactory(
	conn Conn,
	lockFactory lock.LockFactory,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
	durations CheckDurations,
) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,

		secrets:       secrets,
		varSourcePool: varSourcePool,

		planFactory: atc.NewPlanFactory(time.Now().Unix()),

		defaultCheckTimeout:             durations.Timeout,
		defaultCheckInterval:            durations.Interval,
		defaultWithWebhookCheckInterval: durations.IntervalWithWebhook,
	}
}

func (c *checkFactory) TryCreateCheck(ctx context.Context, checkable Checkable, resourceTypes ResourceTypes, from atc.Version, manuallyTriggered bool) (Build, bool, error) {
	logger := lagerctx.FromContext(ctx)

	var err error

	sourceDefaults := atc.Source{}
	parentType, found := resourceTypes.Parent(checkable)
	if found {
		if parentType.Version() == nil {
			return nil, false, fmt.Errorf("resource type '%s' has no version", parentType.Name())
		}
		sourceDefaults = parentType.Defaults()
	} else {
		defaults, found := atc.FindBaseResourceTypeDefaults(checkable.Type())
		if found {
			sourceDefaults = defaults
		}
	}

	interval := c.defaultCheckInterval
	if checkable.HasWebhook() {
		interval = c.defaultWithWebhookCheckInterval
	}
	if checkable.CheckEvery() != nil && !checkable.CheckEvery().Never {
		interval = checkable.CheckEvery().Interval
	}

	if !manuallyTriggered && time.Now().Before(checkable.LastCheckEndTime().Add(interval)) {
		// skip creating the check if its interval hasn't elapsed yet
		return nil, false, nil
	}

	checkPlan := checkable.CheckPlan(from, interval, resourceTypes.Filter(checkable), sourceDefaults)

	plan := c.planFactory.NewPlan(checkPlan)

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
		Where(sq.Eq{"p.paused": false}).
		// filter for resources that are job inputs
		Where(sq.NotEq{"ji.resource_id": nil}).
		// also inlude put-only resources that have errored
		Suffix("OR (b.status IN ('aborted','failed','errored') AND ji.resource_id IS NULL)").
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

func (c *checkFactory) ResourceTypes() ([]ResourceType, error) {
	var resourceTypes []ResourceType

	rows, err := resourceTypesQuery.
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

		resourceTypes = append(resourceTypes, r)
	}

	return resourceTypes, nil
}
