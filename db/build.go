package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
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
	JobName() string
	PipelineName() string
	TeamName() string
	TeamID() int
	Engine() string
	EngineMetadata() string
	Status() Status
	StartTime() time.Time
	EndTime() time.Time
	ReapTime() time.Time
	IsOneOff() bool
	IsScheduled() bool
	IsRunning() bool
	IsManuallyTriggered() bool

	Reload() (bool, error)

	Events(from uint) (EventSource, error)
	SaveEvent(event atc.Event) error

	GetVersionedResources() (SavedVersionedResources, error)
	GetResources() ([]BuildInput, []BuildOutput, error)

	Start(string, string) (bool, error)
	Finish(status Status) error
	MarkAsFailed(cause error) error
	Abort() error
	AbortNotifier() (Notifier, error)

	AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error)

	GetPreparation() (BuildPreparation, bool, error)

	SaveEngineMetadata(engineMetadata string) error

	SaveInput(input BuildInput) (SavedVersionedResource, error)
	SaveOutput(vr VersionedResource, explicit bool) (SavedVersionedResource, error)

	SaveImageResourceVersion(planID atc.PlanID, identifier ResourceCacheIdentifier) error
	GetImageResourceCacheIdentifiers() ([]ResourceCacheIdentifier, error)

	GetConfig() (atc.Config, ConfigVersion, error)

	GetPipeline() (SavedPipeline, error)
}

type build struct {
	id        int
	name      string
	status    Status
	scheduled bool

	jobName      string
	pipelineName string
	pipelineID   int
	teamName     string
	teamID       int

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

func (b *build) JobName() string {
	return b.jobName
}

func (b *build) PipelineName() string {
	return b.pipelineName
}

func (b *build) TeamName() string {
	return b.teamName
}

func (b *build) TeamID() int {
	return b.teamID
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

func (b *build) IsOneOff() bool {
	return b.jobName == ""
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

func (b *build) Abort() error {
	_, err := b.conn.Exec(`
   UPDATE builds
   SET status = 'aborted'
   WHERE id = $1
 `, b.id)
	if err != nil {
		return err
	}

	err = b.bus.Notify(buildAbortChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) AbortNotifier() (Notifier, error) {
	return newConditionNotifier(b.bus, buildAbortChannel(b.id), func() (bool, error) {
		var aborted bool
		err := b.conn.QueryRow(`
			SELECT status = 'aborted'
			FROM builds
			WHERE id = $1
		`, b.id).Scan(&aborted)

		return aborted, err
	})
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

func (b *build) MarkAsFailed(cause error) error {
	err := b.SaveEvent(event.Error{
		Message: cause.Error(),
	})
	if err != nil {
		return err
	}

	return b.Finish(StatusErrored)
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

func (b *build) GetVersionedResources() (SavedVersionedResources, error) {
	return b.getVersionedResources(`
		SELECT vr.id,
			vr.enabled,
			vr.version,
			vr.metadata,
			vr.type,
			r.name,
			r.pipeline_id,
			vr.modified_time
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN build_inputs bi ON bi.build_id = b.id
		INNER JOIN versioned_resources vr ON bi.versioned_resource_id = vr.id
		INNER JOIN resources r ON vr.resource_id = r.id
		WHERE b.id = $1

		UNION ALL

		SELECT vr.id,
			vr.enabled,
			vr.version,
			vr.metadata,
			vr.type,
			r.name,
			r.pipeline_id,
			vr.modified_time
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN build_outputs bo ON bo.build_id = b.id
		INNER JOIN versioned_resources vr ON bo.versioned_resource_id = vr.id
		INNER JOIN resources r ON vr.resource_id = r.id
		WHERE b.id = $1 AND bo.explicit`)
}

func (b *build) GetPreparation() (BuildPreparation, bool, error) {
	if b.IsOneOff() || b.status != StatusPending {
		return BuildPreparation{
			BuildID:             b.id,
			PausedPipeline:      BuildPreparationStatusNotBlocking,
			PausedJob:           BuildPreparationStatusNotBlocking,
			MaxRunningBuilds:    BuildPreparationStatusNotBlocking,
			Inputs:              map[string]BuildPreparationStatus{},
			InputsSatisfied:     BuildPreparationStatusNotBlocking,
			MissingInputReasons: MissingInputReasons{},
		}, true, nil
	}

	var (
		pausedPipeline     bool
		pausedJob          bool
		maxInFlightReached bool
		pipelineID         int
		jobName            string
	)
	err := b.conn.QueryRow(`
			SELECT p.paused, j.paused, j.max_in_flight_reached, j.pipeline_id, j.name
			FROM builds b
			JOIN jobs j
				ON b.job_id = j.id
			JOIN pipelines p
				ON j.pipeline_id = p.id
			WHERE b.id = $1
		`, b.id).Scan(&pausedPipeline, &pausedJob, &maxInFlightReached, &pipelineID, &jobName)
	if err != nil {
		if err == sql.ErrNoRows {
			return BuildPreparation{}, false, nil
		}
		return BuildPreparation{}, false, err
	}

	pausedPipelineStatus := BuildPreparationStatusNotBlocking
	if pausedPipeline {
		pausedPipelineStatus = BuildPreparationStatusBlocking
	}

	pausedJobStatus := BuildPreparationStatusNotBlocking
	if pausedJob {
		pausedJobStatus = BuildPreparationStatusBlocking
	}

	maxInFlightReachedStatus := BuildPreparationStatusNotBlocking
	if maxInFlightReached {
		maxInFlightReachedStatus = BuildPreparationStatusBlocking
	}

	tdbf := NewTeamDBFactory(b.conn, b.bus, b.lockFactory)
	tdb := tdbf.GetTeamDB(b.teamName)
	savedPipeline, found, err := tdb.GetPipelineByName(b.pipelineName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	pdbf := NewPipelineDBFactory(b.conn, b.bus, b.lockFactory)
	pdb := pdbf.Build(savedPipeline)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	jobConfig, found := pdb.Config().Jobs.Lookup(jobName)
	if !found {
		return BuildPreparation{}, false, nil
	}

	configInputs := config.JobInputs(jobConfig)

	nextBuildInputs, found, err := pdb.GetNextBuildInputs(jobName)

	inputsSatisfiedStatus := BuildPreparationStatusBlocking
	inputs := map[string]BuildPreparationStatus{}
	missingInputReasons := MissingInputReasons{}

	if found {
		inputsSatisfiedStatus = BuildPreparationStatusNotBlocking
		for _, buildInput := range nextBuildInputs {
			inputs[buildInput.Name] = BuildPreparationStatusNotBlocking
		}
	} else {
		buildInputs, err := pdb.GetIndependentBuildInputs(jobName)
		if err != nil {
			return BuildPreparation{}, false, err
		}

		for _, configInput := range configInputs {
			found := false
			for _, buildInput := range buildInputs {
				if buildInput.Name == configInput.Name {
					found = true
					break
				}
			}
			if found {
				inputs[configInput.Name] = BuildPreparationStatusNotBlocking
			} else {
				inputs[configInput.Name] = BuildPreparationStatusBlocking
				if len(configInput.Passed) > 0 {
					if configInput.Version != nil && configInput.Version.Pinned != nil {
						_, found, err := pdb.GetVersionedResourceByVersion(configInput.Version.Pinned, configInput.Resource)
						if err != nil {
							return BuildPreparation{}, false, err
						}

						if found {
							missingInputReasons.RegisterPassedConstraint(configInput.Name)
						} else {
							versionJSON, err := json.Marshal(configInput.Version.Pinned)
							if err != nil {
								return BuildPreparation{}, false, err
							}

							missingInputReasons.RegisterPinnedVersionUnavailable(configInput.Name, string(versionJSON))
						}
					} else {
						missingInputReasons.RegisterPassedConstraint(configInput.Name)
					}
				} else {
					if configInput.Version != nil && configInput.Version.Pinned != nil {
						versionJSON, err := json.Marshal(configInput.Version.Pinned)
						if err != nil {
							return BuildPreparation{}, false, err
						}

						missingInputReasons.RegisterPinnedVersionUnavailable(configInput.Name, string(versionJSON))
					} else {
						missingInputReasons.RegisterNoVersions(configInput.Name)
					}
				}
			}
		}
	}

	buildPreparation := BuildPreparation{
		BuildID:             b.id,
		PausedPipeline:      pausedPipelineStatus,
		PausedJob:           pausedJobStatus,
		MaxRunningBuilds:    maxInFlightReachedStatus,
		Inputs:              inputs,
		InputsSatisfied:     inputsSatisfiedStatus,
		MissingInputReasons: missingInputReasons,
	}

	return buildPreparation, true, nil
}

func (b *build) SaveInput(input BuildInput) (SavedVersionedResource, error) {
	row := b.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
	 	INNER JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, input.VersionedResource.PipelineID)

	savedPipeline, err := scanPipeline(row)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	pipelineDBFactory := NewPipelineDBFactory(b.conn, b.bus, b.lockFactory)

	pipelineDB := pipelineDBFactory.Build(savedPipeline)

	return pipelineDB.SaveInput(b.id, input)
}

func (b *build) SaveOutput(vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	row := b.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
	 	INNER JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, vr.PipelineID)

	savedPipeline, err := scanPipeline(row)
	if err != nil {
		return SavedVersionedResource{}, err
	}
	pipelineDBFactory := NewPipelineDBFactory(b.conn, b.bus, b.lockFactory)
	pipelineDB := pipelineDBFactory.Build(savedPipeline)

	return pipelineDB.SaveOutput(b.id, vr, explicit)
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
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	updated, err := checkIfRowsUpdated(tx, `
		UPDATE builds
		SET last_tracked = now()
		WHERE id = $1
			AND now() - last_tracked > ($2 || ' SECONDS')::INTERVAL
	`, b.id, interval.Seconds())
	if err != nil {
		return nil, false, err
	}

	if !updated {
		return nil, false, nil
	}

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

	err = tx.Commit()
	if err != nil {
		lock.Release()
		return nil, false, err
	}

	return lock, true, nil
}

func (b *build) GetConfig() (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := b.conn.QueryRow(`
			SELECT p.config, p.version
			FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			WHERE b.id = $1
		`, b.id).Scan(&configBlob, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, 0, nil
		} else {
			return atc.Config{}, 0, err
		}
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, 0, err
	}

	return config, ConfigVersion(version), nil
}

func (b *build) GetPipeline() (SavedPipeline, error) {
	if b.IsOneOff() {
		return SavedPipeline{}, nil
	}

	row := b.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines p
		INNER JOIN teams t ON t.id = p.team_id
		WHERE p.id = $1
	`, b.pipelineID)

	return scanPipeline(row)
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
