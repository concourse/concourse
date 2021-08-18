package db

import (
	"code.cloudfoundry.org/lager"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"
	"github.com/concourse/concourse/vars"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/propagation"
	"strconv"
	"strings"
	"time"
)

// inMemoryCheckBuild handles in-memory check builds only, thus it just implement
// the necessary function of interface Build.
type inMemoryCheckBuild struct {
	preId            int
	id               int
	checkable        Checkable
	plan             atc.Plan
	resourceId       int
	resourceName     string
	resourceTypeId   int
	resourceTypeName string
	spanContext      SpanContext

	createTime time.Time
	startTime  time.Time
	endTime    time.Time
	status     BuildStatus

	running     bool
	conn        Conn
	lockFactory lock.LockFactory

	// runningInContainer makes a check build really executed in a container on a worker.
	runningInContainer bool
	dbInited           bool

	cacheEvents []atc.Event
	eventIdSeq  util.SequenceGenerator

	cacheAssociatedTeams []string
}

func newRunningInMemoryCheckBuild(conn Conn, lockFactory lock.LockFactory, checkable Checkable, plan atc.Plan, spanContext SpanContext, seqGen util.SequenceGenerator) (*inMemoryCheckBuild, error) {
	build, err := newExistingInMemoryCheckBuildForViewOnly(conn, 0, checkable)
	if err != nil {
		return nil, err
	}

	build.lockFactory = lockFactory
	build.plan = plan
	build.running = true
	build.spanContext = spanContext
	build.preId = seqGen.Next()
	build.eventIdSeq = util.NewSequenceGenerator(0)
	build.createTime = time.Now()
	build.startTime = time.Now()
	build.status = BuildStatusStarted

	build.SaveEvent(event.Status{
		Status: atc.StatusStarted,
		Time:   time.Now().Unix(),
	})

	return build, nil
}

func newExistingInMemoryCheckBuildForViewOnly(conn Conn, buildId int, checkable Checkable) (*inMemoryCheckBuild, error) {
	build := inMemoryCheckBuild{
		id:        buildId,
		conn:      conn,
		checkable: checkable,
		running:   false,
	}

	if resource, ok := checkable.(Resource); ok {
		build.resourceId = resource.ID()
		build.resourceName = resource.Name()
	} else if resourceType, ok := checkable.(ResourceType); ok {
		build.resourceTypeId = resourceType.ID()
		build.resourceTypeName = resourceType.Name()
	} else {
		return nil, errors.New("not supported checkable for in memory check build")
	}

	return &build, nil
}

func (b *inMemoryCheckBuild) RunStateID() string {
	return fmt.Sprintf("in-memory-check-build:%v", b.preId)
}

func (b *inMemoryCheckBuild) ID() int                                 { return b.id }
func (b *inMemoryCheckBuild) Name() string                            { return CheckBuildName }
func (b *inMemoryCheckBuild) TeamID() int                             { return b.checkable.TeamID() }
func (b *inMemoryCheckBuild) TeamName() string                        { return b.checkable.TeamName() }
func (b *inMemoryCheckBuild) PipelineID() int                         { return b.checkable.PipelineID() }
func (b *inMemoryCheckBuild) PipelineName() string                    { return b.checkable.PipelineName() }
func (b *inMemoryCheckBuild) PipelineRef() atc.PipelineRef            { return b.checkable.PipelineRef() }
func (b *inMemoryCheckBuild) Pipeline() (Pipeline, bool, error)       { return b.checkable.Pipeline() }
func (b *inMemoryCheckBuild) ResourceID() int                         { return b.resourceId }
func (b *inMemoryCheckBuild) ResourceName() string                    { return b.resourceName }
func (b *inMemoryCheckBuild) ResourceTypeID() int                     { return b.resourceTypeId }
func (b *inMemoryCheckBuild) Schema() string                          { return schema }
func (b *inMemoryCheckBuild) IsRunning() bool                         { return b.status == BuildStatusStarted }
func (b *inMemoryCheckBuild) IsManuallyTriggered() bool               { return false }
func (b *inMemoryCheckBuild) CreateTime() time.Time                   { return b.createTime }
func (b *inMemoryCheckBuild) StartTime() time.Time                    { return b.startTime }
func (b *inMemoryCheckBuild) EndTime() time.Time                      { return b.endTime }
func (b *inMemoryCheckBuild) Status() BuildStatus                     { return b.status }
func (b *inMemoryCheckBuild) CreatedBy() *string                      { return nil }
func (b *inMemoryCheckBuild) SpanContext() propagation.TextMapCarrier { return b.spanContext }
func (b *inMemoryCheckBuild) PipelineInstanceVars() atc.InstanceVars {
	return b.checkable.PipelineInstanceVars()
}

// JobID returns 0 because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobID() int { return 0 }

// JobName returns an empty string because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobName() string { return "" }

func (b *inMemoryCheckBuild) LagerData() lager.Data {
	data := lager.Data{
		"build":    b.Name(),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}

	if b.preId != 0 {
		data["pre_build_id"] = b.preId
	}

	if b.id != 0 {
		data["build_id"] = b.id
	}

	if b.resourceId != 0 {
		data["resource"] = b.resourceName
	}

	return data
}

func (b *inMemoryCheckBuild) TracingAttrs() tracing.Attrs {
	attrs := tracing.Attrs{
		"build":    b.Name(),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}

	if b.preId != 0 {
		attrs["pre_build_id"] = fmt.Sprintf("%d", b.preId)
	}

	if b.id != 0 {
		attrs["build_id"] = fmt.Sprintf("%d", b.id)
	}

	if b.resourceId != 0 {
		attrs["resource"] = b.resourceName
	}

	return attrs
}

// Reload just does nothing because an in-memory build lives shortly.
func (b *inMemoryCheckBuild) Reload() (bool, error) {
	return true, nil
}

func (b *inMemoryCheckBuild) PrivatePlan() atc.Plan {
	return b.plan
}

// OnCheckBuildStart is a hook point called once a check build starts. For
// in-memory check build, this is a chance to initialize database connection.
func (b *inMemoryCheckBuild) OnCheckBuildStart() error {
	if !b.running {
		return errors.New("not running build cannot start check")
	}

	b.runningInContainer = true

	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}
	defer Rollback(tx)

	if !b.dbInited {
		err := b.initDbStuff(tx)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return b.conn.Bus().Notify(buildEventsChannel(b.id))
}

// AcquireTrackingLock tries to acquire a lock on checkable's ID in order to
// avoid duplicate checks on the same checkable among ATCs and Lidar scan
// intervals.
func (b *inMemoryCheckBuild) AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	if !b.running {
		return nil, false, errors.New("not running build cannot acquire tracking lock")
	}

	var lockId lock.LockID
	if b.ResourceID() != 0 {
		lockId = lock.NewInMemoryCheckBuildTrackingLockID("resource", b.ResourceID())
	} else if b.ResourceTypeID() != 0 {
		lockId = lock.NewInMemoryCheckBuildTrackingLockID("resourceType", b.ResourceTypeID())
	} else {
		return nil, false, errors.New("")
	}

	lock, acquired, err := b.lockFactory.Acquire(
		logger.Session("lock", lager.Data{
			"preBuildId": b.preId,
		}),
		lockId,
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return lock, true, nil
}

func (b *inMemoryCheckBuild) Finish(status BuildStatus) error {
	if !b.running {
		return errors.New("not running build cannot finish")
	}

	if !b.runningInContainer {
		return nil
	}

	b.status = status
	b.endTime = time.Now()

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

	// Release the containers using in this build, so that they can be GC-ed.
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
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = psql.Insert("check_build_events").
		Columns("event_id", "build_id", "type", "version", "payload").
		Values(b.eventIdSeq.Next(), b.id, string(event.EventType()), string(event.Version()), payload).
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
		return errors.New("not running build cannot save event")
	}

	if !b.runningInContainer {
		b.cacheEvents = append(b.cacheEvents, ev)
		return nil
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
	if b.id == 0 {
		return nil, fmt.Errorf("no build event")
	}

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
		func(tx Tx, buildID int) (bool, error) {
			completed := false

			var lastCheckStartTime, lastCheckEndTime pq.NullTime
			err = psql.Select("last_check_start_time", "last_check_end_time").
				From("resource_config_scopes").
				Where(sq.Eq{"last_check_build_id": buildID}).
				RunWith(tx).
				QueryRow().
				Scan(&lastCheckStartTime, &lastCheckEndTime)
			if err != nil {
				if err == sql.ErrNoRows {
					completed = true
				} else {
					return false, err
				}
			}

			if lastCheckStartTime.Valid && lastCheckEndTime.Valid && lastCheckStartTime.Time.Before(lastCheckEndTime.Time) {
				completed = true
			}
			return completed, nil
		},
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
	return b.checkable != nil && b.checkable.BuildSummary() != nil && b.checkable.BuildSummary().PublicPlan != nil
}

func (b *inMemoryCheckBuild) PublicPlan() *json.RawMessage {
	if b.plan.ID != "" {
		return b.plan.Public()
	}

	if b.checkable.BuildSummary() == nil || b.checkable.BuildSummary().PublicPlan == nil {
		return nil
	}
	bytes, err := json.Marshal(b.checkable.BuildSummary().PublicPlan)
	if err != nil {
		return nil
	}
	m := json.RawMessage(bytes)
	return &m
}

// ResourceCacheUser return no-user because a check build may only generate a image
// resource and image resource will be cached by SaveImageResourceVersion.
func (b *inMemoryCheckBuild) ResourceCacheUser() ResourceCacheUser {
	return NoUser()
}

func (b *inMemoryCheckBuild) ContainerOwner(planId atc.PlanID) ContainerOwner {
	return NewInMemoryCheckBuildContainerOwner(b.id, planId, b.TeamID())
}

// SaveImageResourceVersion does nothing. Because if a check use a custom resource
// type, the resource type image's resource cache id will be set in the resource's
// resource config as resource_cache_id, so that the image's resource cache will not
// be GC-ed. As checks run every minute, the resource_config's last_referenced time
// keeps updated, then the image's resource cache will be always retained.
func (b *inMemoryCheckBuild) SaveImageResourceVersion(cache UsedResourceCache) error {
	return nil
}

func (b *inMemoryCheckBuild) AllAssociatedTeamNames() []string {
	if b.cacheAssociatedTeams != nil {
		return b.cacheAssociatedTeams
	}

	rows, err := psql.Select("distinct(t.name)").
		From("resources r").
		LeftJoin("pipelines p on r.pipeline_id = p.id").
		LeftJoin("teams t on p.team_id = t.id").
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

func (b *inMemoryCheckBuild) initDbStuff(tx Tx) error {
	var nextBuildId int
	err := psql.Select("nextval('builds_id_seq'::regclass)").RunWith(tx).QueryRow().Scan(&nextBuildId)
	if err != nil {
		return err
	}
	b.id = nextBuildId

	for _, cachedEv := range b.cacheEvents {
		err := b.saveEvent(tx, cachedEv)
		if err != nil {
			return err
		}
	}

	b.dbInited = true
	b.cacheEvents = []atc.Event{}

	return nil
}

func (b *inMemoryCheckBuild) SyslogTag(origin event.OriginID) string {
	segments := []string{b.TeamName()}

	if b.PipelineID() != 0 {
		segments = append(segments, b.PipelineName())
	}

	if b.ResourceID() != 0 {
		segments = append(segments, b.ResourceName(), strconv.Itoa(b.id))
	} else {
		segments = append(segments, strconv.Itoa(b.id))
	}

	segments = append(segments, origin.String())

	return strings.Join(segments, "/")
}

// As in-memory builds should only be check builds, the following functions
// should never been called, so return false value and errors for them.

func (b *inMemoryCheckBuild) PrototypeID() int      { return 0 }
func (b *inMemoryCheckBuild) PrototypeName() string { return "" }
func (b *inMemoryCheckBuild) ReapTime() time.Time   { return time.Time{} }
func (b *inMemoryCheckBuild) IsScheduled() bool     { return false }
func (b *inMemoryCheckBuild) IsDrained() bool       { return false }
func (b *inMemoryCheckBuild) IsAborted() bool       { return false }
func (b *inMemoryCheckBuild) IsCompleted() bool     { return false }
func (b *inMemoryCheckBuild) InputsReady() bool     { return false }
func (b *inMemoryCheckBuild) RerunOf() int          { return 0 }
func (b *inMemoryCheckBuild) RerunOfName() string   { return "" }
func (b *inMemoryCheckBuild) RerunNumber() int      { return 0 }
func (b *inMemoryCheckBuild) SetDrained(bool) error {
	return errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Delete() (bool, error) {
	return false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) MarkAsAborted() error {
	return errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Interceptible() (bool, error) {
	return false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Preparation() (BuildPreparation, bool, error) {
	return BuildPreparation{}, false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) SetInterceptible(bool) error {
	return errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Artifacts() ([]WorkerArtifact, error) {
	return nil, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Artifact(int) (WorkerArtifact, error) {
	return nil, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Start(atc.Plan) (bool, error) {
	return false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Comment() string {
	return ""
}
func (b *inMemoryCheckBuild) SetComment(string) error {
	return errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Job() (Job, bool, error) {
	return nil, false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) ResourcesChecked() (bool, error) {
	return false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) Resources() ([]BuildInput, []BuildOutput, error) {
	return nil, nil, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) SavePipeline(atc.PipelineRef, int, atc.Config, ConfigVersion, bool) (Pipeline, bool, error) {
	return nil, false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) AdoptInputsAndPipes() ([]BuildInput, bool, error) {
	return nil, false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) AdoptRerunInputsAndPipes() ([]BuildInput, bool, error) {
	return nil, false, errors.New("not implemented for in memory build")
}
func (b *inMemoryCheckBuild) SaveOutput(string, atc.Source, atc.VersionedResourceTypes, atc.Version, ResourceConfigMetadataFields, string, string) error {
	return errors.New("not implemented for in memory build")
}