package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/util"
)

//counterfeiter:generate . Prototype
type Prototype interface {
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

	CheckPlan(planFactory atc.PlanFactory, imagePlanner atc.ImagePlanner, from atc.Version, interval atc.CheckEvery, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) atc.Plan
	CreateBuild(context.Context, bool, atc.Plan) (Build, bool, error)

	CreateInMemoryBuild(context.Context, atc.Plan, util.SequenceGenerator) (Build, error)

	Version() atc.Version

	Reload() (bool, error)
}

type Prototypes []Prototype

func (prototypes Prototypes) Configs() atc.Prototypes {
	var configs atc.Prototypes

	for _, p := range prototypes {
		configs = append(configs, atc.Prototype{
			Name:       p.Name(),
			Type:       p.Type(),
			Source:     p.Source(),
			Defaults:   p.Defaults(),
			Privileged: p.Privileged(),
			CheckEvery: p.CheckEvery(),
			Tags:       p.Tags(),
			Params:     p.Params(),
		})
	}

	return configs
}

var prototypesQuery = psql.Select(
	"pt.id",
	"pt.pipeline_id",
	"pt.name",
	"pt.type",
	"pt.config",
	"rcv.version",
	"p.nonce",
	"p.name",
	"p.instance_vars",
	"t.id",
	"t.name",
	"pt.resource_config_id",
	"ro.id",
	"ro.last_check_start_time",
	"ro.last_check_end_time",
).
	From("prototypes pt").
	Join("pipelines p ON p.id = pt.pipeline_id").
	Join("teams t ON t.id = p.team_id").
	LeftJoin("resource_configs c ON c.id = pt.resource_config_id").
	LeftJoin("resource_config_scopes ro ON ro.resource_config_id = c.id").
	LeftJoin(`LATERAL (
		SELECT rcv.*
		FROM resource_config_versions rcv
		WHERE rcv.resource_config_scope_id = ro.id
		ORDER BY rcv.check_order DESC
		LIMIT 1
	) AS rcv ON true`).
	Where(sq.Eq{"pt.active": true})

type prototype struct {
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
	version               atc.Version
	checkEvery            *atc.CheckEvery
	lastCheckStartTime    time.Time
	lastCheckEndTime      time.Time
}

func (p *prototype) ID() int                       { return p.id }
func (p *prototype) TeamID() int                   { return p.teamID }
func (p *prototype) TeamName() string              { return p.teamName }
func (p *prototype) Name() string                  { return p.name }
func (p *prototype) Type() string                  { return p.type_ }
func (p *prototype) Privileged() bool              { return p.privileged }
func (p *prototype) CheckEvery() *atc.CheckEvery   { return p.checkEvery }
func (p *prototype) CheckTimeout() string          { return "" }
func (p *prototype) LastCheckStartTime() time.Time { return p.lastCheckStartTime }
func (p *prototype) LastCheckEndTime() time.Time   { return p.lastCheckEndTime }
func (p *prototype) Source() atc.Source            { return p.source }
func (p *prototype) Defaults() atc.Source          { return p.defaults }
func (p *prototype) Params() atc.Params            { return p.params }
func (p *prototype) Tags() atc.Tags                { return p.tags }
func (p *prototype) ResourceConfigID() int         { return p.resourceConfigID }
func (p *prototype) ResourceConfigScopeID() int    { return p.resourceConfigScopeID }

func (p *prototype) Version() atc.Version              { return p.version }
func (p *prototype) CurrentPinnedVersion() atc.Version { return nil }

func (p *prototype) HasWebhook() bool { return false }

func newEmptyPrototype(conn DbConn, lockFactory lock.LockFactory) *prototype {
	return &prototype{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
}

func (p *prototype) Reload() (bool, error) {
	row := prototypesQuery.Where(sq.Eq{"pt.id": p.id}).RunWith(p.conn).QueryRow()

	err := scanPrototype(p, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (p *prototype) SetResourceConfigScope(scope ResourceConfigScope) error {
	return setResourceConfigScopeForPrototype(p.conn, scope, sq.Eq{"id": p.id})
}

func setResourceConfigScopeForPrototype(conn sq.Runner, scope ResourceConfigScope, pred interface{}, args ...interface{}) error {
	_, err := psql.Update("prototypes").
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

func (p *prototype) CheckPlan(planFactory atc.PlanFactory, imagePlanner atc.ImagePlanner, from atc.Version, interval atc.CheckEvery, sourceDefaults atc.Source, skipInterval bool, skipIntervalRecursively bool) atc.Plan {
	plan := planFactory.NewPlan(atc.CheckPlan{
		Name:   p.Name(),
		Type:   p.Type(),
		Source: sourceDefaults.Merge(p.Source()),
		Tags:   p.Tags(),

		FromVersion: from,
		Interval:    interval,

		Prototype: p.Name(),

		SkipInterval: skipInterval,
	})

	plan.Check.TypeImage = imagePlanner.ImageForType(plan.ID, p.Type(), p.Tags(), skipInterval && skipIntervalRecursively)
	return plan
}

func (p *prototype) CreateBuild(ctx context.Context, manuallyTriggered bool, plan atc.Plan) (Build, bool, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	build := newEmptyBuild(p.conn, p.lockFactory)
	err = createStartedBuild(tx, build, startedBuildArgs{
		Name:              CheckBuildName,
		PipelineID:        p.pipelineID,
		TeamID:            p.teamID,
		Plan:              plan,
		ManuallyTriggered: manuallyTriggered,
		SpanContext:       NewSpanContext(ctx),
	})
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	err = p.conn.Bus().Notify(atc.ComponentBuildTracker)
	if err != nil {
		return nil, false, err
	}

	_, err = build.Reload()
	if err != nil {
		return nil, false, err
	}

	return build, true, nil
}

func (p *prototype) CreateInMemoryBuild(context.Context, atc.Plan, util.SequenceGenerator) (Build, error) {
	return nil, errors.New("prototype not supporting in-memory build yet")
}

func scanPrototype(p *prototype, row scannable) error {
	var (
		configJSON                           sql.NullString
		rcsID, version, nonce                sql.NullString
		lastCheckStartTime, lastCheckEndTime sql.NullTime
		pipelineInstanceVars                 sql.NullString
		resourceConfigID                     sql.NullInt64
	)

	err := row.Scan(&p.id, &p.pipelineID, &p.name, &p.type_, &configJSON, &version, &nonce, &p.pipelineName, &pipelineInstanceVars, &p.teamID, &p.teamName, &resourceConfigID, &rcsID, &lastCheckStartTime, &lastCheckEndTime)
	if err != nil {
		return err
	}

	p.lastCheckStartTime = lastCheckStartTime.Time
	p.lastCheckEndTime = lastCheckEndTime.Time

	if version.Valid {
		err = json.Unmarshal([]byte(version.String), &p.version)
		if err != nil {
			return err
		}
	}

	es := p.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	var config atc.Prototype
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
		config = atc.Prototype{}
	}

	p.source = config.Source
	p.defaults = config.Defaults
	p.params = config.Params
	p.privileged = config.Privileged
	p.tags = config.Tags
	p.checkEvery = config.CheckEvery

	if resourceConfigID.Valid {
		p.resourceConfigID = int(resourceConfigID.Int64)
	}

	if rcsID.Valid {
		p.resourceConfigScopeID, err = strconv.Atoi(rcsID.String)
		if err != nil {
			return err
		}
	}

	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &p.pipelineInstanceVars)
		if err != nil {
			return err
		}
	}

	return nil
}
