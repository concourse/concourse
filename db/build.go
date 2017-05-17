package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/event"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusStarted   Status = "started"
	StatusAborted   Status = "aborted"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusErrored   Status = "errored"
)

const buildColumns = "id, name, job_id, team_id, status, manually_triggered, scheduled, engine, engine_metadata, start_time, end_time, reap_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_id, b.team_id, b.status, b.manually_triggered, b.scheduled, b.engine, b.engine_metadata, b.start_time, b.end_time, b.reap_time, j.name as job_name, p.id as pipeline_id, p.name as pipeline_name, t.name as team_name"

//go:generate counterfeiter . Build

type Build interface {
	ID() int
	Name() string
	JobID() int
	JobName() string
	PipelineID() int
	PipelineName() string
	TeamID() int
	TeamName() string
	Engine() string
	EngineMetadata() string
	Status() Status
	StartTime() time.Time
	EndTime() time.Time
	ReapTime() time.Time
	IsScheduled() bool
	IsRunning() bool
	IsManuallyTriggered() bool

	Reload() (bool, error)

	Events(from uint) (EventSource, error)
	SaveEvent(event atc.Event) error

	GetResources() ([]BuildInput, []BuildOutput, error)

	Start(string, string) (bool, error)
	Finish(status Status) error

	AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error)

	SaveEngineMetadata(engineMetadata string) error

	SaveImageResourceVersion(planID atc.PlanID, identifier ResourceCacheIdentifier) error
	GetImageResourceCacheIdentifiers() ([]ResourceCacheIdentifier, error)
}

type build struct {
	id        int
	name      string
	status    Status
	scheduled bool

	teamID   int
	teamName string

	pipelineID   int
	pipelineName string
	jobID        int
	jobName      string

	isManuallyTriggered bool

	engine         string
	engineMetadata string

	startTime time.Time
	endTime   time.Time
	reapTime  time.Time

	conn Conn
	bus  *notificationsBus

	lockFactory lock.LockFactory
}

func (b *build) ID() int {
	return b.id
}

func (b *build) Name() string {
	return b.name
}

func (b *build) JobID() int {
	return b.jobID
}

func (b *build) JobName() string {
	return b.jobName
}

func (b *build) PipelineID() int {
	return b.pipelineID
}

func (b *build) PipelineName() string {
	return b.pipelineName
}

func (b *build) TeamID() int {
	return b.teamID
}

func (b *build) TeamName() string {
	return b.teamName
}

func (b *build) IsManuallyTriggered() bool {
	return b.isManuallyTriggered
}

func (b *build) Engine() string {
	return b.engine
}

func (b *build) EngineMetadata() string {
	return b.engineMetadata
}

func (b *build) StartTime() time.Time {
	return b.startTime
}

func (b *build) EndTime() time.Time {
	return b.endTime
}

func (b *build) ReapTime() time.Time {
	return b.reapTime
}

func (b *build) Status() Status {
	return b.status
}

func (b *build) IsScheduled() bool {
	return b.scheduled
}

func (b *build) IsRunning() bool {
	switch b.status {
	case StatusPending, StatusStarted:
		return true
	default:
		return false
	}
}

func (b *build) Reload() (bool, error) {
	buildFactory := newBuildFactory(b.conn, b.bus, b.lockFactory)
	newBuild, found, err := buildFactory.ScanBuild(b.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.id = $1
	`, b.id))
	if err != nil {
		return false, err
	}

	if !found {
		return found, nil
	}

	b.id = newBuild.ID()
	b.name = newBuild.Name()
	b.status = newBuild.Status()
	b.scheduled = newBuild.IsScheduled()
	b.engine = newBuild.Engine()
	b.engineMetadata = newBuild.EngineMetadata()
	b.startTime = newBuild.StartTime()
	b.endTime = newBuild.EndTime()
	b.reapTime = newBuild.ReapTime()
	b.teamName = newBuild.TeamName()
	b.teamID = newBuild.TeamID()
	b.jobName = newBuild.JobName()
	b.jobID = newBuild.JobID()
	b.pipelineName = newBuild.PipelineName()

	return found, err
}

func (b *build) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(b.bus, buildEventsChannel(b.id), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	table := fmt.Sprintf("team_build_events_%d", b.teamID)
	if b.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", b.pipelineID)
	}

	return newSQLDBBuildEventSource(
		b.id,
		table,
		b.conn,
		notifier,
		from,
	), nil
}

func (b *build) Start(engine, metadata string) (bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var startTime time.Time

	err = tx.QueryRow(`
		UPDATE builds
		SET status = 'started', start_time = now(), engine = $2, engine_metadata = $3
		WHERE id = $1
		AND status = 'pending'
		RETURNING start_time
	`, b.id, engine, metadata).Scan(&startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	err = b.saveEvent(tx, event.Status{
		Status: atc.StatusStarted,
		Time:   startTime.Unix(),
	})
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	err = b.bus.Notify(buildEventsChannel(b.id))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (b *build) Finish(status Status) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var endTime time.Time

	err = tx.QueryRow(`
		UPDATE builds
		SET status = $2, end_time = now(), completed = true
		WHERE id = $1
		RETURNING end_time
	`, b.id, string(status)).Scan(&endTime)
	if err != nil {
		return err
	}

	err = b.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(status),
		Time:   endTime.Unix(),
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

	err = b.bus.Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) SaveEvent(event atc.Event) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = b.saveEvent(tx, event)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = b.bus.Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) GetResources() ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := b.conn.Query(`
		SELECT i.name, r.name, v.type, v.version, v.metadata, r.pipeline_id,
		NOT EXISTS (
			SELECT 1
			FROM build_inputs ci, builds cb
			WHERE versioned_resource_id = v.id
			AND cb.job_id = b.job_id
			AND ci.build_id = cb.id
			AND ci.build_id < b.id
		)
		FROM versioned_resources v, build_inputs i, builds b, resources r
		WHERE b.id = $1
		AND i.build_id = b.id
		AND i.versioned_resource_id = v.id
    AND r.id = v.resource_id
		AND NOT EXISTS (
			SELECT 1
			FROM build_outputs o
			WHERE o.versioned_resource_id = v.id
			AND o.build_id = i.build_id
			AND o.explicit
		)
	`, b.id)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var inputName string
		var vr VersionedResource
		var firstOccurrence bool

		var version, metadata string
		err := rows.Scan(&inputName, &vr.Resource, &vr.Type, &version, &metadata, &vr.PipelineID, &firstOccurrence)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		inputs = append(inputs, BuildInput{
			Name:              inputName,
			VersionedResource: vr,
			FirstOccurrence:   firstOccurrence,
		})
	}

	rows, err = b.conn.Query(`
		SELECT r.name, v.type, v.version, v.metadata, r.pipeline_id
		FROM versioned_resources v, build_outputs o, builds b, resources r
		WHERE b.id = $1
		AND o.build_id = b.id
		AND o.versioned_resource_id = v.id
    AND r.id = v.resource_id
		AND o.explicit
	`, b.id)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var vr VersionedResource

		var version, metadata string
		err := rows.Scan(&vr.Resource, &vr.Type, &version, &metadata, &vr.PipelineID)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(version), &vr.Version)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(metadata), &vr.Metadata)
		if err != nil {
			return nil, nil, err
		}

		outputs = append(outputs, BuildOutput{
			VersionedResource: vr,
		})
	}

	return inputs, outputs, nil
}

func (b *build) getVersionedResources(resourceRequest string) (SavedVersionedResources, error) {
	rows, err := b.conn.Query(resourceRequest, b.id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedVersionedResources := SavedVersionedResources{}

	for rows.Next() {
		var versionedResource SavedVersionedResource
		var versionJSON []byte
		var metadataJSON []byte
		err = rows.Scan(&versionedResource.ID, &versionedResource.Enabled, &versionJSON, &metadataJSON, &versionedResource.Type, &versionedResource.Resource, &versionedResource.PipelineID, &versionedResource.ModifiedTime)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(versionJSON, &versionedResource.Version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(metadataJSON, &versionedResource.Metadata)
		if err != nil {
			return nil, err
		}

		savedVersionedResources = append(savedVersionedResources, versionedResource)
	}

	return savedVersionedResources, nil
}

func (b *build) SaveEngineMetadata(engineMetadata string) error {
	_, err := b.conn.Exec(`
		UPDATE builds
		SET engine_metadata = $2
		WHERE id = $1
	`, b.id, engineMetadata)
	if err != nil {
		return err
	}

	return nil
}

func (b *build) SaveImageResourceVersion(planID atc.PlanID, identifier ResourceCacheIdentifier) error {
	version, err := json.Marshal(identifier.ResourceVersion)
	if err != nil {
		return err
	}

	result, err := b.conn.Exec(`
		UPDATE image_resource_versions
		SET version = $1, resource_hash = $4
		WHERE build_id = $2 AND plan_id = $3
	`, version, b.id, string(planID), identifier.ResourceHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		_, err := b.conn.Exec(`
			INSERT INTO image_resource_versions(version, build_id, plan_id, resource_hash)
			VALUES ($1, $2, $3, $4)
		`, version, b.id, string(planID), identifier.ResourceHash)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *build) GetImageResourceCacheIdentifiers() ([]ResourceCacheIdentifier, error) {
	rows, err := b.conn.Query(`
  	SELECT version, resource_hash
  	FROM image_resource_versions
  	WHERE build_id = $1
  `, b.id)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var identifiers []ResourceCacheIdentifier

	for rows.Next() {
		var identifier ResourceCacheIdentifier
		var marshalledVersion []byte

		err := rows.Scan(&marshalledVersion, &identifier.ResourceHash)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(marshalledVersion, &identifier.ResourceVersion)
		if err != nil {
			return nil, err
		}

		identifiers = append(identifiers, identifier)
	}

	return identifiers, nil
}

func (b *build) AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	lock := b.lockFactory.NewLock(
		logger.Session("lock", lager.Data{
			"build_id": b.id,
		}),
		lock.NewBuildTrackingLockID(b.id),
	)

	acquired, err := lock.Acquire()
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return lock, true, nil
}

func newConditionNotifier(bus *notificationsBus, channel string, cond func() (bool, error)) (Notifier, error) {
	notified, err := bus.Listen(channel)
	if err != nil {
		return nil, err
	}

	notifier := &conditionNotifier{
		cond:    cond,
		bus:     bus,
		channel: channel,

		notified: notified,
		notify:   make(chan struct{}, 1),

		stop: make(chan struct{}),
	}

	go notifier.watch()

	return notifier, nil
}

func (b *build) saveEvent(tx Tx, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	table := fmt.Sprintf("team_build_events_%d", b.teamID)
	if b.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", b.pipelineID)
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO %s (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, table, buildEventSeq(b.id)), b.id, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
