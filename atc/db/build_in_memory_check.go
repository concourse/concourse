package db

import (
	"code.cloudfoundry.org/lager"
	"encoding/json"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"go.opentelemetry.io/otel/propagation"
	"time"
)

// inMemoryCheckBuild handles in-memory check builds only, thus it just implement
// the necessary function of interface Build.
type inMemoryCheckBuild struct {
	id               int
	checkable        Checkable
	plan             atc.Plan
	createTime       time.Time
	resourceId       int
	resourceName     string
	resourceTypeId   int
	resourceTypeName string
	spanContext      SpanContext

	running bool
	conn    Conn

	cacheAssociatedTeams []string
}

func newRunningInMemoryCheckBuild(conn Conn, checkable Checkable, plan atc.Plan, spanContext SpanContext) (*inMemoryCheckBuild, error) {
	tx, err := conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	var nextBuildId int
	err = psql.Select("nextval('builds_id_seq'::regclass)").RunWith(tx).QueryRow().Scan(&nextBuildId)
	if err != nil {
		return nil, err
	}

	err = createBuildEventSeq(tx, nextBuildId)
	if err != nil {
		return nil, err
	}

	build := inMemoryCheckBuild{
		id:          nextBuildId,
		checkable:   checkable,
		plan:        plan,
		spanContext: spanContext,
		createTime:  time.Now(),
		running:     true,
		conn:        conn,
	}

	if resource, ok := checkable.(Resource); ok {
		build.resourceId = resource.ID()
		build.resourceName = resource.Name()
	} else if resourceType, ok := checkable.(ResourceType); ok {
		build.resourceTypeId = resourceType.ID()
		build.resourceTypeName = resourceType.Name()
	} else {
		return nil, fmt.Errorf("invalid checkable")
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &build, nil
}

func newExistingInMemoryCheckBuild(conn Conn, buildId int, checkable Checkable) *inMemoryCheckBuild {
	return &inMemoryCheckBuild{
		id:        buildId,
		conn:      conn,
		checkable: checkable,
		running:   false,
	}
}

func (b *inMemoryCheckBuild) ID() int                                 { return b.id }
func (b *inMemoryCheckBuild) Name() string                            { return CheckBuildName }
func (b *inMemoryCheckBuild) CreateTime() time.Time                   { return b.createTime }
func (b *inMemoryCheckBuild) TeamID() int                             { return b.checkable.TeamID() }
func (b *inMemoryCheckBuild) TeamName() string                        { return b.checkable.TeamName() }
func (b *inMemoryCheckBuild) PipelineID() int                         { return b.checkable.PipelineID() }
func (b *inMemoryCheckBuild) PipelineName() string                    { return b.checkable.PipelineName() }
func (b *inMemoryCheckBuild) PipelineRef() atc.PipelineRef            { return b.checkable.PipelineRef() }
func (b *inMemoryCheckBuild) Pipeline() (Pipeline, bool, error)       { return b.checkable.Pipeline() }
func (b *inMemoryCheckBuild) ResourceID() int                         { return b.resourceId }
func (b *inMemoryCheckBuild) ResourceName() string                    { return b.resourceName }
func (b *inMemoryCheckBuild) ResourceTypeID() int                     { return b.resourceTypeId }
func (b *inMemoryCheckBuild) ResourceTypeName() string                { return b.resourceTypeName }
func (b *inMemoryCheckBuild) Schema() string                          { return schema }
func (b *inMemoryCheckBuild) IsRunning() bool                         { return b.running }
func (b *inMemoryCheckBuild) IsManuallyTriggered() bool               { return false }
func (b *inMemoryCheckBuild) PrivatePlan() atc.Plan                   { return b.plan }
func (b *inMemoryCheckBuild) SpanContext() propagation.TextMapCarrier { return b.spanContext }
func (b *inMemoryCheckBuild) PipelineInstanceVars() atc.InstanceVars {
	return b.checkable.PipelineInstanceVars()
}

// JobID returns 0 because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobID() int { return 0 }

// JobName returns an empty string because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobName() string { return "" }

func (b *inMemoryCheckBuild) IsNewerThanLastCheckOf(input Resource) bool {
	return b.createTime.After(input.LastCheckEndTime())
}

func (b *inMemoryCheckBuild) LagerData() lager.Data {
	data := lager.Data{
		"build":    b.ID(),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}

	if b.resourceId != 0 {
		data["resource"] = b.resourceName
	}

	if b.resourceTypeId != 0 {
		data["resourceType"] = b.resourceTypeName
	}

	return data
}

func (b *inMemoryCheckBuild) TracingAttrs() tracing.Attrs {
	attrs := tracing.Attrs{
		"build":    fmt.Sprintf("%d", b.ID()),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}

	if b.resourceId != 0 {
		attrs["resource"] = b.resourceName
	}

	if b.resourceTypeId != 0 {
		attrs["resourceType"] = b.resourceTypeName
	}

	return attrs
}

// Reload just reloads the embedded checkable.
func (b *inMemoryCheckBuild) Reload() (bool, error) {
	return b.checkable.Reload()
}

// AcquireTrackingLock returns a noop lock because in-memory check build runs
// on only one ATC, thus no need to use a lock to sync among ATCs.
func (b *inMemoryCheckBuild) AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	return lock.NoopLock{}, true, nil
}

func (b *inMemoryCheckBuild) Finish(status BuildStatus) error {
	if !b.running {
		panic("not a running in-memory-check-build")
	}

	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	err = b.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(status),
		Time:   time.Now().Unix(),
	})
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventSeq(b.id)))
	if err != nil {
		return err
	}

	_, err = psql.Update("containers").
		Set("in_memory_check_build_id", nil).
		Where(sq.Eq{"in_memory_check_build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *inMemoryCheckBuild) saveEvent(tx Tx, event atc.Event) error {
	if !b.running {
		panic("not a running in-memory-check-build")
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = psql.Insert("check_build_events").
		Columns("event_id", "build_id", "type", "version", "payload").
		Values(sq.Expr("nextval('"+buildEventSeq(b.id)+"')"), b.id, string(event.EventType()), string(event.Version()), payload).
		RunWith(tx).
		Exec()
	return err
}

func (b *inMemoryCheckBuild) Variables(logger lager.Logger, secrets creds.Secrets, varSourcePool creds.VarSourcePool) (vars.Variables, error) {
	pipeline, found, err := b.Pipeline()
	if err != nil {
		return nil, fmt.Errorf("failed to find pipeline: %w", err)
	}
	if !found {
		return nil, errors.New("pipeline not found")
	}

	return pipeline.Variables(logger, secrets, varSourcePool)
}

func (b *inMemoryCheckBuild) SaveEvent(ev atc.Event) error {
	if !b.running {
		panic("not a running in-memory-check-build")
	}

	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	err = b.saveEvent(tx, ev)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return b.conn.Bus().Notify(buildEventsChannel(b.id))
}

func (b *inMemoryCheckBuild) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(b.conn.Bus(), buildEventsChannel(b.id), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return newBuildEventSource(
		b.id,
		"check_build_events",
		b.conn,
		notifier,
		from,
		true,
	), nil
}

// AbortNotifier returns NoopNotifier because there is no way to abort a in-memory
// check build. Say a in-memory build may run on ATC-a, but abort-build API call
// might be received by ATC-b, there is not a channel for ATC-b to tell ATC-a to
// mark the in-memory build as aborted. If we really want to abort a in-memory
// check build in future, it might need to add a new table "aborted-in-memory-builds"
// and API insert in-memory build id to the table, and AbortNotifier watches the
// table to see if current build should be aborted.
func (b *inMemoryCheckBuild) AbortNotifier() (Notifier, error) {
	return newNoopNotifier(), nil
}

func (b *inMemoryCheckBuild) HasPlan() bool {
	if b.plan.ID != "" {
		return true
	}
	resource, ok := b.checkable.(Resource)
	return ok && resource.BuildSummary() != nil && resource.BuildSummary().PublicPlan != nil
}

func (b *inMemoryCheckBuild) PublicPlan() *json.RawMessage {
	if b.plan.ID != "" {
		return b.plan.Public()
	}

	resource, ok := b.checkable.(Resource)
	if !ok || resource.BuildSummary() == nil || resource.BuildSummary().PublicPlan == nil {
		return nil
	}
	bytes, err := json.Marshal(resource.BuildSummary().PublicPlan)
	if err != nil {
		return nil
	}
	m := json.RawMessage(bytes)
	return &m
}

func (b *inMemoryCheckBuild) ResourceCacheUser() ResourceCacheUser {
	return NoUser()
}

func (b *inMemoryCheckBuild) ContainerOwner(planId atc.PlanID) ContainerOwner {
	return NewInMemoryCheckBuildContainerOwner(b.ID())
}

// SaveImageResourceVersion does nothing as a resource check doesn't belong to any job.
func (b *inMemoryCheckBuild) SaveImageResourceVersion(cache UsedResourceCache) error {
	return nil
}

func (b *inMemoryCheckBuild) AllAssociatedTeamNames() []string {
	if b.cacheAssociatedTeams != nil {
		return b.cacheAssociatedTeams
	}

	rows, err := sq.Select("distinct(t.name)").
		From("resources r").
		LeftJoin("teams t on r.team_id == t.id").
		Where(sq.Eq{"r.resource_config_scope_id": b.checkable.ResourceConfigScopeID()}).
		RunWith(b.conn).
		Query()
	if err != nil {
		return []string{b.checkable.TeamName()}
	}
	defer Close(rows)

	var teamNames []string
	for rows.Next() {
		var teamName string
		err := rows.Scan(&teamName)
		if err != nil {
			return teamNames
		}
		teamNames = append(teamNames, teamName)
	}
	b.cacheAssociatedTeams = teamNames

	return b.cacheAssociatedTeams
}

// === No implemented functions ===

func (b *inMemoryCheckBuild) PrototypeID() int                             { panic("not-implemented") }
func (b *inMemoryCheckBuild) PrototypeName() string                        { panic("not-implemented") }
func (b *inMemoryCheckBuild) StartTime() time.Time                         { panic("not-implemented") }
func (b *inMemoryCheckBuild) EndTime() time.Time                           { panic("not-implemented") }
func (b *inMemoryCheckBuild) ReapTime() time.Time                          { panic("not-implemented") }
func (b *inMemoryCheckBuild) Status() BuildStatus                          { panic("not-implemented") }
func (b *inMemoryCheckBuild) IsScheduled() bool                            { panic("not-implemented") }
func (b *inMemoryCheckBuild) IsDrained() bool                              { panic("not-implemented") }
func (b *inMemoryCheckBuild) IsAborted() bool                              { panic("not-implemented") }
func (b *inMemoryCheckBuild) IsCompleted() bool                            { panic("not-implemented") }
func (b *inMemoryCheckBuild) InputsReady() bool                            { panic("not-implemented") }
func (b *inMemoryCheckBuild) RerunOf() int                                 { panic("not-implemented") }
func (b *inMemoryCheckBuild) RerunOfName() string                          { panic("not-implemented") }
func (b *inMemoryCheckBuild) RerunNumber() int                             { panic("not-implemented") }
func (b *inMemoryCheckBuild) CreatedBy() *string                           { panic("not-implemented") }
func (b *inMemoryCheckBuild) SetDrained(bool) error                        { panic("not-implemented") }
func (b *inMemoryCheckBuild) Delete() (bool, error)                        { panic("not-implemented") }
func (b *inMemoryCheckBuild) MarkAsAborted() error                         { panic("not-implemented") }
func (b *inMemoryCheckBuild) Interceptible() (bool, error)                 { panic("not-implemented") }
func (b *inMemoryCheckBuild) Preparation() (BuildPreparation, bool, error) { panic("not-implemented") }
func (b *inMemoryCheckBuild) SetInterceptible(b2 bool) error               { panic("not-implemented") }
func (b *inMemoryCheckBuild) Artifacts() ([]WorkerArtifact, error)         { panic("not-implemented") }
func (b *inMemoryCheckBuild) Artifact(int) (WorkerArtifact, error)         { panic("not-implemented") }
func (b *inMemoryCheckBuild) Start(atc.Plan) (bool, error)                 { panic("not-implemented") }
func (b *inMemoryCheckBuild) SyslogTag(id event.OriginID) string           { panic("not-implemented") }
func (b *inMemoryCheckBuild) Comment() string                              { panic("not-implemented") }
func (b *inMemoryCheckBuild) SetComment(string) error                      { panic("not-implemented") }
func (b *inMemoryCheckBuild) Job() (Job, bool, error)                      { panic("not-implemented") }
func (b *inMemoryCheckBuild) ResourcesChecked() (bool, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) Resources() ([]BuildInput, []BuildOutput, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) SavePipeline(atc.PipelineRef, int, atc.Config, ConfigVersion, bool) (Pipeline, bool, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) AdoptInputsAndPipes() ([]BuildInput, bool, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) AdoptRerunInputsAndPipes() ([]BuildInput, bool, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) SaveOutput(string, atc.Source, atc.VersionedResourceTypes, atc.Version, ResourceConfigMetadataFields, string, string) error {
	panic("not-implemented")
}
