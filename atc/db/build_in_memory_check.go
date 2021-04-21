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
	"os"
	"time"
)

type inMemoryCheckBuild struct {
	id               int
	checkable        Checkable
	plan             atc.Plan
	createTime       time.Time
	resourceId       int
	resourceName     string
	resourceTypeId   int
	resourceTypeName string

	conn Conn
}

func (b *inMemoryCheckBuild) TeamID() int {
	return b.checkable.TeamID()
}

func (b *inMemoryCheckBuild) TeamName() string {
	return b.checkable.TeamName()
}

func (b *inMemoryCheckBuild) PipelineID() int {
	return b.checkable.PipelineID()
}

func (b *inMemoryCheckBuild) PipelineName() string {
	return b.checkable.PipelineName()
}

func (b *inMemoryCheckBuild) PipelineInstanceVars() atc.InstanceVars {
	return b.checkable.PipelineInstanceVars()
}

func (b *inMemoryCheckBuild) PipelineRef() atc.PipelineRef {
	return b.checkable.PipelineRef()
}

func (b *inMemoryCheckBuild) Pipeline() (Pipeline, bool, error) {
	return b.checkable.Pipeline()
}

func (b *inMemoryCheckBuild) LagerData() lager.Data {
	return lager.Data{
		"build":    b.ID(),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}
}

func (b *inMemoryCheckBuild) TracingAttrs() tracing.Attrs {
	return tracing.Attrs{
		"build":    fmt.Sprintf("%d", b.ID()),
		"team":     b.TeamName(),
		"pipeline": b.PipelineName(),
	}
}

// Reload does nothing.
func (b *inMemoryCheckBuild) Reload() (bool, error) {
	return true, nil
}

// AcquireTrackingLock returns a noop lock because in-memory check build runs
// on only one ATC, thus no need to use a lock to sync among ATCs.
func (b *inMemoryCheckBuild) AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	return lock.NoopLock{}, true, nil
}

// Finish does nothing for resource type check. For resource check, it will
// update in-memory-check-build info to table resources, and save a finish
// event.
func (b *inMemoryCheckBuild) Finish(status BuildStatus) error {
	if b.resourceId == 0 {
		return nil
	}

	fmt.Fprintf(os.Stderr, "EVAN:finish - %d\n", b.id)

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

func (b *inMemoryCheckBuild) AbortNotifier() (Notifier, error) {
	return newNoopNotifier(), nil
}

func (b *inMemoryCheckBuild) SpanContext() propagation.TextMapCarrier {
	return SpanContext{}
}

func (b *inMemoryCheckBuild) IsManuallyTriggered() bool {
	return false
}

func (b *inMemoryCheckBuild) PrivatePlan() atc.Plan {
	return b.plan
}

func (b *inMemoryCheckBuild) HasPlan() bool            {
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

func (b *inMemoryCheckBuild) ID() int { return b.id }

func (b *inMemoryCheckBuild) Name() string { return CheckBuildName }

// JobID returns 0 because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobID() int { return 0 }

// JobName returns an empty string because check build doesn't belong to any job.
func (b *inMemoryCheckBuild) JobName() string { return "" }

func (b *inMemoryCheckBuild) CreateTime() time.Time { return b.createTime }

func (b *inMemoryCheckBuild) IsNewerThanLastCheckOf(input Resource) bool {
	return b.createTime.After(input.LastCheckEndTime())
}

// IsRunning returns true as a in-memory check build only runs once.
func (b *inMemoryCheckBuild) IsRunning() bool {
	return true
}

func (b *inMemoryCheckBuild) Schema() string {
	return schema
}

func (b *inMemoryCheckBuild) ResourceID() int          { return b.resourceId }
func (b *inMemoryCheckBuild) ResourceName() string     { return b.resourceName }
func (b *inMemoryCheckBuild) ResourceTypeID() int      { return b.resourceTypeId }
func (b *inMemoryCheckBuild) ResourceTypeName() string { return b.resourceTypeName }

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
func (b *inMemoryCheckBuild) ResourcesChecked() (bool, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) Resources() ([]BuildInput, []BuildOutput, error) {
	panic("not-implemented")
}
func (b *inMemoryCheckBuild) SaveImageResourceVersion(cache UsedResourceCache) error {
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
