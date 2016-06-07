package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . BuildDBFactory

type BuildDBFactory interface {
	GetBuildDB(build Build) BuildDB
}

func NewBuildDBFactory(conn Conn, bus *notificationsBus) BuildDBFactory {
	return &buildDBFactory{
		conn: conn,
		bus:  bus,
	}
}

type buildDBFactory struct {
	conn Conn
	bus  *notificationsBus
}

func (f *buildDBFactory) GetBuildDB(build Build) BuildDB {
	return &buildDB{
		build:      build,
		buildID:    build.ID,
		pipelineID: build.PipelineID,
		conn:       f.conn,
		bus:        f.bus,
	}
}

//go:generate counterfeiter . BuildDB

type BuildDB interface {
	Get() (Build, bool, error)
	GetID() int
	GetName() string
	GetJobName() string
	GetPipelineName() string
	GetTeamName() string
	GetEngineMetadata() string

	Events(from uint) (EventSource, error)

	Start(string, string) (bool, error)
	Finish(status Status) error
	MarkAsFailed(cause error) error
	AbortNotifier() (Notifier, error)
	SaveEvent(event atc.Event) error

	LeaseScheduling(logger lager.Logger, interval time.Duration) (Lease, bool, error)

	GetPreparation() (BuildPreparation, bool, error)

	SaveEngineMetadata(engineMetadata string) error

	SaveInput(input BuildInput) (SavedVersionedResource, error)
	SaveOutput(vr VersionedResource, explicit bool) (SavedVersionedResource, error)

	SaveImageResourceVersion(planID atc.PlanID, identifier ResourceCacheIdentifier) error
}

type buildDB struct {
	buildID    int
	pipelineID int
	build      Build
	conn       Conn
	bus        *notificationsBus

	buildPrepHelper buildPreparationHelper
}

func (db *buildDB) Get() (Build, bool, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
		WHERE b.id = $1
	`, db.buildID))
}

func (db *buildDB) GetID() int {
	return db.buildID
}

func (db *buildDB) GetName() string {
	return db.build.Name
}

func (db *buildDB) GetJobName() string {
	return db.build.JobName
}

func (db *buildDB) GetPipelineName() string {
	return db.build.PipelineName
}

func (db *buildDB) GetTeamName() string {
	return db.build.TeamName
}

func (db *buildDB) GetEngineMetadata() string {
	return db.build.EngineMetadata
}

func (db *buildDB) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(db.bus, buildEventsChannel(db.buildID), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	table := "build_events"
	if db.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", db.pipelineID)
	}

	return newSQLDBBuildEventSource(
		db.buildID,
		table,
		db.conn,
		notifier,
		from,
	), nil
}

func (db *buildDB) Start(engine, metadata string) (bool, error) {
	tx, err := db.conn.Begin()
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
	`, db.buildID, engine, metadata).Scan(&startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	err = db.saveEvent(tx, event.Status{
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

	err = db.bus.Notify(buildEventsChannel(db.buildID))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (db *buildDB) AbortNotifier() (Notifier, error) {
	return newConditionNotifier(db.bus, buildAbortChannel(db.buildID), func() (bool, error) {
		var aborted bool
		err := db.conn.QueryRow(`
			SELECT status = 'aborted'
			FROM builds
			WHERE id = $1
		`, db.buildID).Scan(&aborted)

		return aborted, err
	})
}

func (db *buildDB) Finish(status Status) error {
	tx, err := db.conn.Begin()
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
	`, db.buildID, string(status)).Scan(&endTime)
	if err != nil {
		return err
	}

	err = db.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(status),
		Time:   endTime.Unix(),
	})
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventSeq(db.buildID)))
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = db.bus.Notify(buildEventsChannel(db.buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *buildDB) MarkAsFailed(cause error) error {
	err := db.SaveEvent(event.Error{
		Message: cause.Error(),
	})
	if err != nil {
		return err
	}

	return db.Finish(StatusErrored)
}

func (db *buildDB) SaveEvent(event atc.Event) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = db.saveEvent(tx, event)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = db.bus.Notify(buildEventsChannel(db.buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *buildDB) LeaseScheduling(logger lager.Logger, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: logger.Session("lease", lager.Data{
			"build_id": db.buildID,
		}),
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
					AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
			`, db.buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
			`, db.buildID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (db *buildDB) GetPreparation() (BuildPreparation, bool, error) {
	return db.buildPrepHelper.GetBuildPreparation(db.conn, db.buildID)
}

func (db *buildDB) SaveInput(input BuildInput) (SavedVersionedResource, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines
		WHERE id = $1
	`, input.VersionedResource.PipelineID)

	savedPipeline, err := scanPipeline(row)
	if err != nil {
		return SavedVersionedResource{}, err
	}
	pipelineDBFactory := NewPipelineDBFactory(db.conn, db.bus)
	pipelineDB := pipelineDBFactory.Build(savedPipeline)

	return pipelineDB.SaveInput(db.buildID, input)
}

func (db *buildDB) SaveOutput(vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	row := db.conn.QueryRow(`
		SELECT `+pipelineColumns+`
		FROM pipelines
		WHERE id = $1
	`, vr.PipelineID)

	savedPipeline, err := scanPipeline(row)
	if err != nil {
		return SavedVersionedResource{}, err
	}
	pipelineDBFactory := NewPipelineDBFactory(db.conn, db.bus)
	pipelineDB := pipelineDBFactory.Build(savedPipeline)

	return pipelineDB.SaveOutput(db.buildID, vr, explicit)
}

func (db *buildDB) SaveEngineMetadata(engineMetadata string) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET engine_metadata = $2
		WHERE id = $1
	`, db.buildID, engineMetadata)
	if err != nil {
		return err
	}

	return nil
}

func (db *buildDB) SaveImageResourceVersion(planID atc.PlanID, identifier ResourceCacheIdentifier) error {
	version, err := json.Marshal(identifier.ResourceVersion)
	if err != nil {
		return err
	}

	result, err := db.conn.Exec(`
		UPDATE image_resource_versions
		SET version = $1, resource_hash = $4
		WHERE build_id = $2 AND plan_id = $3
	`, version, db.buildID, string(planID), identifier.ResourceHash)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		_, err := db.conn.Exec(`
			INSERT INTO image_resource_versions(version, build_id, plan_id, resource_hash)
			VALUES ($1, $2, $3, $4)
		`, version, db.buildID, string(planID), identifier.ResourceHash)
		if err != nil {
			return err
		}
	}

	return nil
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

func (db *buildDB) saveEvent(tx Tx, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	table := "build_events"
	if db.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", db.pipelineID)
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO %s (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, table, buildEventSeq(db.buildID)), db.buildID, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
}
