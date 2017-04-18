package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db/lock"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
)

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusStarted   BuildStatus = "started"
	BuildStatusAborted   BuildStatus = "aborted"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusErrored   BuildStatus = "errored"
)

var buildsQuery = psql.Select("b.id, b.name, b.job_id, b.team_id, b.status, b.manually_triggered, b.scheduled, b.engine, b.engine_metadata, b.start_time, b.end_time, b.reap_time, j.name, p.id, p.name, t.name").
	From("builds b").
	JoinClause("LEFT OUTER JOIN jobs j ON b.job_id = j.id").
	JoinClause("LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id").
	JoinClause("LEFT OUTER JOIN teams t ON b.team_id = t.id")

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
	Status() BuildStatus
	StartTime() time.Time
	EndTime() time.Time
	ReapTime() time.Time
	IsManuallyTriggered() bool
	IsScheduled() bool

	IsRunning() bool

	Reload() (bool, error)

	Interceptible() (bool, error)
	AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error)
	Preparation() (BuildPreparation, bool, error)

	Start(string, string) (bool, error)
	SaveStatus(s BuildStatus) error
	SetInterceptible(bool) error

	Events(uint) (EventSource, error)
	SaveEvent(event atc.Event) error

	SaveInput(input BuildInput) error
	SaveOutput(vr VersionedResource, explicit bool) error

	GetVersionedResources() (SavedVersionedResources, error)
	SaveImageResourceVersion(planID atc.PlanID, resourceVersion atc.Version, resourceHash string) error

	Pipeline() (Pipeline, bool, error)

	Finish(s BuildStatus) error
	Delete() (bool, error)
	Abort() error
	AbortNotifier() (Notifier, error)
}

type build struct {
	id        int
	name      string
	status    BuildStatus
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

	conn        Conn
	lockFactory lock.LockFactory
}

var ErrBuildDisappeared = errors.New("build-disappeared-from-db")

func (b *build) ID() int                   { return b.id }
func (b *build) Name() string              { return b.name }
func (b *build) JobID() int                { return b.jobID }
func (b *build) JobName() string           { return b.jobName }
func (b *build) PipelineID() int           { return b.pipelineID }
func (b *build) PipelineName() string      { return b.pipelineName }
func (b *build) TeamID() int               { return b.teamID }
func (b *build) TeamName() string          { return b.teamName }
func (b *build) IsManuallyTriggered() bool { return b.isManuallyTriggered }
func (b *build) Engine() string            { return b.engine }
func (b *build) EngineMetadata() string    { return b.engineMetadata }
func (b *build) StartTime() time.Time      { return b.startTime }
func (b *build) EndTime() time.Time        { return b.endTime }
func (b *build) ReapTime() time.Time       { return b.reapTime }
func (b *build) Status() BuildStatus       { return b.status }
func (b *build) IsScheduled() bool         { return b.scheduled }

func (b *build) IsRunning() bool {
	switch b.status {
	case BuildStatusPending, BuildStatusStarted:
		return true
	default:
		return false
	}
}

func (b *build) IsOneOff() bool {
	return b.jobID == 0
}

func (b *build) Reload() (bool, error) {
	row := buildsQuery.Where(sq.Eq{"b.id": b.id}).
		RunWith(b.conn).
		QueryRow()

	err := scanBuild(b, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (b *build) Interceptible() (bool, error) {
	var interceptible bool

	err := psql.Select("interceptible").
		From("builds").
		Where(sq.Eq{
		"id": b.id,
	}).
		RunWith(b.conn).
		QueryRow().Scan(&interceptible)

	if err != nil {
		return true, err
	}

	return interceptible, nil
}

func (b *build) SetInterceptible(i bool) error {
	rows, err := psql.Update("builds").
		Set("interceptible", i).
		Where(sq.Eq{
		"id": b.id,
	}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrBuildDisappeared
	}

	return nil

}

func (b *build) Start(engine, metadata string) (bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var startTime time.Time

	err = psql.Update("builds").
		Set("status", "started").
		Set("start_time", sq.Expr("now()")).
		Set("engine", engine).
		Set("engine_metadata", metadata).
		Where(sq.Eq{
		"id":     b.id,
		"status": "pending",
	}).
		Suffix("RETURNING start_time").
		RunWith(tx).
		QueryRow().
		Scan(&startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
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

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (b *build) SaveStatus(s BuildStatus) error {
	rows, err := psql.Update("builds").
		Set("status", string(s)).
		Where(sq.Eq{
		"id": b.id,
	}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if affected == 0 {
		return ErrBuildDisappeared
	}

	return nil
}

func (b *build) Finish(s BuildStatus) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var endTime time.Time

	err = psql.Update("builds").
		Set("status", string(s)).
		Set("end_time", sq.Expr("now()")).
		Set("completed", true).
		Where(sq.Eq{"id": b.id}).
		Suffix("RETURNING end_time").
		RunWith(tx).
		QueryRow().
		Scan(&endTime)
	if err != nil {
		return err
	}

	err = b.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(s),
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

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) Delete() (bool, error) {
	rows, err := psql.Delete("builds").
		Where(sq.Eq{
		"id": b.id,
	}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return false, err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return false, err
	}

	if affected == 0 {
		return false, ErrBuildDisappeared
	}

	return true, nil
}

func (b *build) Abort() error {
	_, err := psql.Update("builds").
		Set("status", BuildStatusAborted).
		Where(sq.Eq{"id": b.id}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return err
	}

	err = b.conn.Bus().Notify(buildAbortChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) AbortNotifier() (Notifier, error) {
	return newConditionNotifier(b.conn.Bus(), buildAbortChannel(b.id), func() (bool, error) {
		var aborted bool
		err := psql.Select("status = 'aborted'").
			From("builds").
			Where(sq.Eq{"id": b.id}).
			RunWith(b.conn).
			QueryRow().
			Scan(&aborted)

		return aborted, err
	})
}

func (b *build) Pipeline() (Pipeline, bool, error) {
	if b.pipelineID == 0 {
		return nil, false, nil
	}

	row := pipelinesQuery.
		Where(sq.Eq{"p.id": b.pipelineID}).
		RunWith(b.conn).
		QueryRow()

	pipeline := newPipeline(b.conn, b.lockFactory)
	err := scanPipeline(pipeline, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return pipeline, true, nil
}

func (b *build) SaveImageResourceVersion(planID atc.PlanID, resourceVersion atc.Version, resourceHash string) error {
	version, err := json.Marshal(resourceVersion)
	if err != nil {
		return err
	}

	return safeCreateOrUpdate(
		b.conn,
		func(tx Tx) (sql.Result, error) {
			return psql.Insert("image_resource_versions").
				Columns("version", "build_id", "plan_id", "resource_hash").
				Values(version, b.id, string(planID), resourceHash).
				RunWith(tx).
				Exec()
		},
		func(tx Tx) (sql.Result, error) {
			return psql.Update("image_resource_versions").
				Set("version", version).
				Set("resource_hash", resourceHash).
				Where(sq.Eq{
				"build_id": b.id,
				"plan_id":  string(planID),
			}).
				RunWith(tx).
				Exec()
		},
	)
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

func (b *build) Preparation() (BuildPreparation, bool, error) {
	if b.jobID == 0 || b.status != BuildStatusPending {
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
	err := psql.Select("p.paused, j.paused, j.max_in_flight_reached, j.pipeline_id, j.name").
		From("builds b").
		Join("jobs j ON b.job_id = j.id").
		Join("pipelines p ON j.pipeline_id = p.id").
		Where(sq.Eq{"b.id": b.id}).
		RunWith(b.conn).
		QueryRow().
		Scan(&pausedPipeline, &pausedJob, &maxInFlightReached, &pipelineID, &jobName)
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

	tf := NewTeamFactory(b.conn, b.lockFactory)
	t, found, err := tf.FindTeam(b.teamName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	pipeline, found, err := t.Pipeline(b.pipelineName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	jobConfig, found := pipeline.Config().Jobs.Lookup(jobName)
	if !found {
		return BuildPreparation{}, false, nil
	}

	configInputs := config.JobInputs(jobConfig)

	nextBuildInputs, found, err := pipeline.NextBuildInputs(jobName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

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

func (b *build) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(b.conn.Bus(), buildEventsChannel(b.id), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	table := fmt.Sprintf("team_build_events_%d", b.teamID)
	if b.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", b.pipelineID)
	}

	return newBuildEventSource(
		b.id,
		table,
		b.conn,
		notifier,
		from,
	), nil
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

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) SaveInput(input BuildInput) error {
	if b.pipelineID == 0 {
		return nil
	}

	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	row := pipelinesQuery.
		Where(sq.Eq{"p.id": b.pipelineID}).
		RunWith(tx).
		QueryRow()

	pipeline := &pipeline{conn: b.conn, lockFactory: b.lockFactory}
	err = scanPipeline(pipeline, row)
	if err != nil {
		return err
	}

	err = pipeline.saveInputTx(tx, b.id, input)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (b *build) SaveOutput(vr VersionedResource, explicit bool) error {
	if b.pipelineID == 0 {
		return nil
	}

	row := pipelinesQuery.
		Where(sq.Eq{"p.id": b.pipelineID}).
		RunWith(b.conn).
		QueryRow()
	pipeline := &pipeline{conn: b.conn, lockFactory: b.lockFactory}
	err := scanPipeline(pipeline, row)
	if err != nil {
		return err
	}

	return pipeline.saveOutput(b.id, vr, explicit)
}

func (b *build) GetVersionedResources() (SavedVersionedResources, error) {
	return b.getVersionedResources(`
		SELECT vr.id,
			vr.enabled,
			vr.version,
			vr.metadata,
			vr.type,
			r.name,
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
			vr.modified_time
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN build_outputs bo ON bo.build_id = b.id
		INNER JOIN versioned_resources vr ON bo.versioned_resource_id = vr.id
		INNER JOIN resources r ON vr.resource_id = r.id
		WHERE b.id = $1 AND bo.explicit`)
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
		err = rows.Scan(&versionedResource.ID, &versionedResource.Enabled, &versionJSON, &metadataJSON, &versionedResource.Type, &versionedResource.Resource, &versionedResource.ModifiedTime)
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

func createBuildEventSeq(tx Tx, buildid int) error {
	_, err := tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(buildid)))
	return err
}

func buildEventSeq(buildid int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildid)
}

func scanBuild(b *build, row scannable) error {
	var (
		jobID, pipelineID                             sql.NullInt64
		engine, engineMetadata, jobName, pipelineName sql.NullString
		startTime, endTime, reapTime                  pq.NullTime

		status string
	)

	err := row.Scan(&b.id, &b.name, &jobID, &b.teamID, &status, &b.isManuallyTriggered, &b.scheduled, &engine, &engineMetadata, &startTime, &endTime, &reapTime, &jobName, &pipelineID, &pipelineName, &b.teamName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}

		return err
	}

	b.status = BuildStatus(status)
	b.jobName = jobName.String
	b.jobID = int(jobID.Int64)
	b.pipelineName = pipelineName.String
	b.pipelineID = int(pipelineID.Int64)
	b.engine = engine.String
	b.engineMetadata = engineMetadata.String
	b.startTime = startTime.Time
	b.endTime = endTime.Time
	b.reapTime = reapTime.Time

	return nil
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
	_, err = psql.Insert(table).
		Columns("event_id", "build_id", "type", "version", "payload").
		Values(sq.Expr("nextval('"+buildEventSeq(b.id)+"')"), b.id, string(event.EventType()), string(event.Version()), payload).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	return nil
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}
