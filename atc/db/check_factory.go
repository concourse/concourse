package db

import (
	"context"
	"database/sql"
	"encoding/json"
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

	CheckPlan(planFactory atc.PlanFactory, imagePlanner ImagePlanner, varSourceConfigs atc.VarSourceConfigs, from atc.Version, interval time.Duration, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) (atc.Plan, error)
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)
}

//counterfeiter:generate . CheckFactory
type CheckFactory interface {
	TryCreateCheck(context.Context, Checkable, ResourceTypes, atc.VarSourceConfigs, atc.Version, bool, bool) (Build, bool, error)
	Resources() ([]Resource, error)
	ResourceTypes() ([]ResourceType, error)
	VarSources() (map[int]atc.VarSourceConfigs, error)
}

type checkFactory struct {
	conn        Conn
	lockFactory lock.LockFactory

	secrets creds.Secrets

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
	durations CheckDurations,
) CheckFactory {
	return &checkFactory{
		conn:        conn,
		lockFactory: lockFactory,

		secrets: secrets,

		planFactory: atc.NewPlanFactory(time.Now().Unix()),

		defaultCheckTimeout:             durations.Timeout,
		defaultCheckInterval:            durations.Interval,
		defaultWithWebhookCheckInterval: durations.IntervalWithWebhook,
	}
}

func (c *checkFactory) TryCreateCheck(ctx context.Context, checkable Checkable, resourceTypes ResourceTypes, varSourceConfigs atc.VarSourceConfigs, from atc.Version, manuallyTriggered bool, skipIntervalRecursively bool) (Build, bool, error) {
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

	interval := c.defaultCheckInterval
	if checkable.HasWebhook() {
		interval = c.defaultWithWebhookCheckInterval
	}
	if checkable.CheckEvery() != nil && !checkable.CheckEvery().Never {
		interval = checkable.CheckEvery().Interval
	}

	skipInterval := manuallyTriggered
	if !skipInterval && time.Now().Before(checkable.LastCheckEndTime().Add(interval)) {
		// skip creating the check if its interval hasn't elapsed yet
		return nil, false, nil
	}

	deserializedResourceTypes := resourceTypes.Filter(checkable).Deserialize()
	if deserializedResourceTypes == nil {
		// If there are no resource types, set it to a zero length list of resource
		// types. The reason behind this is because we wrap the resource types
		// object in an ImagePlanner interface, and this will panic if the resource
		// types is nil.
		deserializedResourceTypes = atc.ResourceTypes{}
	}

	plan, err := checkable.CheckPlan(c.planFactory, deserializedResourceTypes, varSourceConfigs, from, interval, sourceDefaults, skipInterval, skipIntervalRecursively)
	if err != nil {
		return nil, false, fmt.Errorf("check plan: %w", err)
	}

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

func (c *checkFactory) ResourceTypes() ([]ResourceType, error) {
	var resourceTypes []ResourceType

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

		resourceTypes = append(resourceTypes, r)
	}

	return resourceTypes, nil
}

func (c *checkFactory) VarSources() (map[int]atc.VarSourceConfigs, error) {
	rows, err := psql.Select("id", "var_sources", "nonce").
		From("pipelines").
		Where(sq.And{
			sq.Eq{"paused": false},
		}).
		RunWith(c.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	varSourcesPerPipeline := map[int]atc.VarSourceConfigs{}
	for rows.Next() {
		var (
			pipelineID      int
			varSourcesBytes sql.NullString
			nonce           sql.NullString
			nonceStr        *string
		)

		err = rows.Scan(&pipelineID, &varSourcesBytes, &nonce)
		if err != nil {
			return nil, err
		}

		if nonce.Valid {
			nonceStr = &nonce.String
		}

		var pipelineVarSources atc.VarSourceConfigs
		decryptedVarSource, err := c.conn.EncryptionStrategy().Decrypt(varSourcesBytes.String, nonceStr)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(decryptedVarSource), &pipelineVarSources)
		if err != nil {
			return nil, err
		}

		if len(pipelineVarSources) > 0 {
			varSourcesPerPipeline[pipelineID] = pipelineVarSources
		}
	}

	return varSourcesPerPipeline, nil
}
