package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
)

type SQLDB struct {
	logger lager.Logger

	conn *sql.DB
	bus  *notificationsBus
}

const buildColumns = "id, name, job_id, status, scheduled, engine, engine_metadata, start_time, end_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_id, b.status, b.scheduled, b.engine, b.engine_metadata, b.start_time, b.end_time, j.name as job_name, p.name as pipeline_name"

func NewSQL(
	logger lager.Logger,
	sqldbConnection *sql.DB,
	bus *notificationsBus,
) *SQLDB {
	return &SQLDB{
		logger: logger,

		conn: sqldbConnection,
		bus:  bus,
	}
}

func (db *SQLDB) GetPipelineByName(pipelineName string) (SavedPipeline, error) {
	row := db.conn.QueryRow(`
		SELECT id, name, config, version, paused
		FROM pipelines
		WHERE name = $1
	`, pipelineName)

	return scanPipeline(row)
}

func (db *SQLDB) GetAllActivePipelines() ([]SavedPipeline, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, config, version, paused
		FROM pipelines
		ORDER BY ordering
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	pipelines := []SavedPipeline{}

	for rows.Next() {

		pipeline, err := scanPipeline(rows)

		if err != nil {
			return nil, err
		}

		pipelines = append(pipelines, pipeline)
	}

	return pipelines, nil
}

func (db *SQLDB) OrderPipelines(pipelineNames []string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	var pipelineCount int

	err = tx.QueryRow(`
			SELECT COUNT(1)
			FROM pipelines
	`).Scan(&pipelineCount)

	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE pipelines
		SET ordering = $1
	`, pipelineCount+1)

	if err != nil {
		return err
	}

	for i, name := range pipelineNames {
		_, err = tx.Exec(`
			UPDATE pipelines
			SET ordering = $1
			WHERE name = $2
		`, i, name)

		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *SQLDB) GetConfigByBuildID(buildID int) (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
			SELECT p.config, p.version
			FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
			WHERE b.ID = $1
		`, buildID).Scan(&configBlob, &version)
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

func (db *SQLDB) GetConfig(pipelineName string) (atc.Config, ConfigVersion, error) {
	var configBlob []byte
	var version int
	err := db.conn.QueryRow(`
		SELECT config, version
		FROM pipelines
		WHERE name = $1
	`, pipelineName).Scan(&configBlob, &version)
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

type PipelinePausedState string

const (
	PipelinePaused   PipelinePausedState = "paused"
	PipelineUnpaused PipelinePausedState = "unpaused"
	PipelineNoChange PipelinePausedState = "nochange"
)

func (state PipelinePausedState) Bool() *bool {
	yes := true
	no := false

	switch state {
	case PipelinePaused:
		return &yes
	case PipelineUnpaused:
		return &no
	case PipelineNoChange:
		return nil
	default:
		panic("unknown pipeline state")
	}
}

func (db *SQLDB) SaveConfig(pipelineName string, config atc.Config, from ConfigVersion, pausedState PipelinePausedState) (bool, error) {
	payload, err := json.Marshal(config)
	if err != nil {
		return false, err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	var existingConfig int
	err = tx.QueryRow(`
		SELECT COUNT(1)
		FROM pipelines
		WHERE name = $1
	`, pipelineName).Scan(&existingConfig)
	if err != nil {
		return false, err
	}

	var result sql.Result

	if pausedState == PipelineNoChange {
		result, err = tx.Exec(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq')
				WHERE name = $2
					AND version = $3
			`, payload, pipelineName, from)
	} else {
		result, err = tx.Exec(`
				UPDATE pipelines
				SET config = $1, version = nextval('config_version_seq'), paused = $2
				WHERE name = $3
					AND version = $4
			`, payload, pausedState.Bool(), pipelineName, from)
	}

	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	created := false

	if rows == 0 {
		if existingConfig == 0 {
			// If there is no state to change from then start the pipeline out as
			// paused.
			if pausedState == PipelineNoChange {
				pausedState = PipelinePaused
			}

			created = true

			_, err := tx.Exec(`
			INSERT INTO pipelines (name, config, version, ordering, paused)
			VALUES ($1, $2, nextval('config_version_seq'), (SELECT COUNT(1) + 1 FROM pipelines), $3)
		`, pipelineName, payload, pausedState.Bool())
			if err != nil {
				return false, err
			}
		} else {
			return false, ErrConfigComparisonFailed
		}
	}

	return created, tx.Commit()
}

func (db *SQLDB) CreatePipe(pipeGUID string, url string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO pipes(id, url)
		VALUES ($1, $2)
	`, pipeGUID, url)

	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) GetPipe(pipeGUID string) (Pipe, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Pipe{}, err
	}

	defer tx.Rollback()

	var pipe Pipe

	err = tx.QueryRow(`
		SELECT id, coalesce(url, '') AS url
		FROM pipes
		WHERE id = $1
	`, pipeGUID).Scan(&pipe.ID, &pipe.URL)

	if err != nil {
		return Pipe{}, err
	}
	err = tx.Commit()
	if err != nil {
		return Pipe{}, err
	}

	return pipe, nil
}

func (db *SQLDB) GetAllBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		ORDER BY b.id DESC
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetAllStartedBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.status = 'started'
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetBuild(buildID int) (Build, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.id = $1
	`, buildID))
}

func (db *SQLDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	build, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, status)
		VALUES (nextval('one_off_name'), 'pending')
		RETURNING ` + buildColumns + `, null, null
	`))
	if err != nil {
		return Build{}, err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(build.ID)))
	if err != nil {
		return Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (db *SQLDB) StartBuild(buildID int, engine, metadata string) (bool, error) {
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
	`, buildID, engine, metadata).Scan(&startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}

		return false, err
	}

	err = db.saveBuildEvent(tx, buildID, event.Status{
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

	// doesn't really need to be in transaction
	_, err = db.conn.Exec("NOTIFY " + buildEventsChannel(buildID))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (db *SQLDB) FinishBuild(buildID int, status Status) error {
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
	`, buildID, string(status)).Scan(&endTime)
	if err != nil {
		return err
	}

	err = db.saveBuildEvent(tx, buildID, event.Status{
		Status: atc.BuildStatus(status),
		Time:   endTime.Unix(),
	})
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventSeq(buildID)))
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// doesn't really need to be in transaction
	_, err = db.conn.Exec("NOTIFY " + buildEventsChannel(buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) ErrorBuild(buildID int, cause error) error {
	err := db.SaveBuildEvent(buildID, event.Error{
		Message: cause.Error(),
	})
	if err != nil {
		return err
	}

	return db.FinishBuild(buildID, StatusErrored)
}

func (db *SQLDB) SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error) {
	pipelineDBFactory := NewPipelineDBFactory(db.logger, db.conn, db.bus, db)
	pipelineDB, err := pipelineDBFactory.BuildWithName(input.VersionedResource.PipelineName)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return pipelineDB.SaveBuildInput(buildID, input)
}

func (db *SQLDB) SaveBuildOutput(buildID int, vr VersionedResource) (SavedVersionedResource, error) {
	pipelineDBFactory := NewPipelineDBFactory(db.logger, db.conn, db.bus, db)
	pipelineDB, err := pipelineDBFactory.BuildWithName(vr.PipelineName)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return pipelineDB.SaveBuildOutput(buildID, vr)
}

func (db *SQLDB) SaveBuildEngineMetadata(buildID int, engineMetadata string) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET engine_metadata = $2
		WHERE id = $1
	`, buildID, engineMetadata)
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) GetBuildEvents(buildID int, from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(db.bus, buildEventsChannel(buildID), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return newSQLDBBuildEventSource(
		buildID,
		db.conn,
		notifier,
		from,
	), nil
}

func (db *SQLDB) AbortBuild(buildID int) error {
	_, err := db.conn.Exec(`
		UPDATE builds
		SET status = 'aborted'
		WHERE id = $1
	`, buildID)
	if err != nil {
		return err
	}

	_, err = db.conn.Exec("NOTIFY " + buildAbortChannel(buildID))
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLDB) AbortNotifier(buildID int) (Notifier, error) {
	return newConditionNotifier(db.bus, buildAbortChannel(buildID), func() (bool, error) {
		var aborted bool
		err := db.conn.QueryRow(`
			SELECT status = 'aborted'
			FROM builds
			WHERE id = $1
		`, buildID).Scan(&aborted)

		return aborted, err
	})
}

func (db *SQLDB) SaveBuildEvent(buildID int, event atc.Event) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = db.saveBuildEvent(tx, buildID, event)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	// doesn't really need to be in transaction
	_, err = db.conn.Exec("NOTIFY " + buildEventsChannel(buildID))
	if err != nil {
		return err
	}

	return nil
}

type nonOneRowAffectedError struct {
	RowsAffected int64
}

func (err nonOneRowAffectedError) Error() string {
	return fmt.Sprintf("expected 1 row to be updated; got %d", err.RowsAffected)
}

func (db *SQLDB) acquireLock(lockType string, locks []NamedLock) (Lock, error) {
	params := []interface{}{}
	refs := []string{}
	for i, lock := range locks {
		params = append(params, lock.Name())
		refs = append(refs, fmt.Sprintf("$%d", i+1))

		_, err := db.conn.Exec(`
			INSERT INTO locks (name)
			VALUES ($1)
		`, lock.Name())
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok {
				if pqErr.Code.Class().Name() == "integrity_constraint_violation" {
					// unique violation is ok; no way to atomically upsert
					continue
				}
			}

			return nil, err
		}
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return nil, err
	}

	result, err := tx.Exec(`
		SELECT 1 FROM locks
		WHERE name IN (`+strings.Join(refs, ",")+`)
		FOR `+lockType+`
	`, params...)
	if err != nil {
		tx.Commit()
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		tx.Commit()
		return nil, err
	}

	if rowsAffected == 0 {
		tx.Commit()
		return nil, ErrLockRowNotPresentOrAlreadyDeleted
	}

	return &txLock{tx, db, locks}, nil
}

func (db *SQLDB) acquireLockLoop(lockType string, lock []NamedLock) (Lock, error) {
	for {
		lock, err := db.acquireLock(lockType, lock)
		if err != ErrLockRowNotPresentOrAlreadyDeleted {
			return lock, err
		}
	}
}

func (db *SQLDB) AcquireWriteLockImmediately(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("UPDATE NOWAIT", lock)
}

func (db *SQLDB) AcquireWriteLock(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("UPDATE", lock)
}

func (db *SQLDB) AcquireReadLock(lock []NamedLock) (Lock, error) {
	return db.acquireLockLoop("SHARE", lock)
}

func (db *SQLDB) ListLocks() ([]string, error) {
	rows, err := db.conn.Query("SELECT name FROM locks")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	locks := []string{}

	for rows.Next() {
		var name string
		err := rows.Scan(&name)
		if err != nil {
			return nil, err
		}

		locks = append(locks, name)
	}

	return locks, nil
}

func (db *SQLDB) SaveWorker(info WorkerInfo, ttl time.Duration) error {
	resourceTypes, err := json.Marshal(info.ResourceTypes)
	if err != nil {
		return err
	}

	tags, err := json.Marshal(info.Tags)
	if err != nil {
		return err
	}

	if ttl == 0 {
		result, err := db.conn.Exec(`
			UPDATE workers
			SET expires = NULL, active_containers = $2, resource_types = $3, platform = $4, tags = $5
			WHERE addr = $1
		`, info.Addr, info.ActiveContainers, resourceTypes, info.Platform, tags)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected == 0 {
			_, err := db.conn.Exec(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags)
				VALUES ($1, NULL, $2, $3, $4, $5)
			`, info.Addr, info.ActiveContainers, resourceTypes, info.Platform, tags)
			if err != nil {
				return err
			}
		}

		return nil
	} else {
		interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

		result, err := db.conn.Exec(`
			UPDATE workers
			SET expires = NOW() + $2::INTERVAL, active_containers = $3, resource_types = $4, platform = $5, tags = $6
			WHERE addr = $1
		`, info.Addr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected == 0 {
			_, err := db.conn.Exec(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags)
				VALUES ($1, NOW() + $2::INTERVAL, $3, $4, $5, $6)
			`, info.Addr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func (db *SQLDB) Workers() ([]WorkerInfo, error) {
	// reap expired workers
	_, err := db.conn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	if err != nil {
		return nil, err
	}

	// select remaining workers
	rows, err := db.conn.Query(`
		SELECT addr, active_containers, resource_types, platform, tags
		FROM workers
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	infos := []WorkerInfo{}
	for rows.Next() {
		info := WorkerInfo{}

		var resourceTypes []byte
		var tags []byte

		err := rows.Scan(&info.Addr, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(resourceTypes, &info.ResourceTypes)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(tags, &info.Tags)
		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

type txLock struct {
	tx         *sql.Tx
	db         *SQLDB
	namedLocks []NamedLock
}

func (lock *txLock) release() error {
	return lock.tx.Commit()
}

func (lock *txLock) cleanup() error {
	lockNames := []interface{}{}
	refs := []string{}
	for i, l := range lock.namedLocks {
		lockNames = append(lockNames, l.Name())
		refs = append(refs, fmt.Sprintf("$%d", i+1))
	}

	cleanupLock, err := lock.db.acquireLock("UPDATE NOWAIT", lock.namedLocks)
	if err != nil {
		return nil
	}

	// acquireLock cannot return *txLock as that is a non-nil interface type when it fails
	internalLock := cleanupLock.(*txLock)

	_, err = internalLock.tx.Exec(`
		DELETE FROM locks
		WHERE name IN (`+strings.Join(refs, ",")+`)
	`, lockNames...)

	return internalLock.release()
}

func (lock *txLock) Release() error {
	err := lock.release()
	if err != nil {
		return err
	}

	return lock.cleanup()
}

func (db *SQLDB) saveBuildEvent(tx *sql.Tx, buildID int, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO build_events (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, buildEventSeq(buildID)), buildID, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

type scannable interface {
	Scan(destinations ...interface{}) error
}

func scanPipeline(rows scannable) (SavedPipeline, error) {
	var id int
	var name string
	var configBlob []byte
	var version int
	var paused bool

	err := rows.Scan(&id, &name, &configBlob, &version, &paused)
	if err != nil {
		return SavedPipeline{}, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return SavedPipeline{}, err
	}

	return SavedPipeline{
		ID:     id,
		Paused: paused,
		Pipeline: Pipeline{
			Name:    name,
			Config:  config,
			Version: ConfigVersion(version),
		},
	}, nil
}

func scanBuild(row scannable) (Build, error) {
	var id int
	var name string
	var jobID sql.NullInt64
	var status string
	var scheduled bool
	var engine, engineMetadata, jobName, pipelineName sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime

	err := row.Scan(&id, &name, &jobID, &status, &scheduled, &engine, &engineMetadata, &startTime, &endTime, &jobName, &pipelineName)
	if err != nil {
		if err == sql.ErrNoRows {
			return Build{}, ErrNoBuild
		}

		return Build{}, err
	}

	build := Build{
		ID:        id,
		Name:      name,
		Status:    Status(status),
		Scheduled: scheduled,

		Engine:         engine.String,
		EngineMetadata: engineMetadata.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
	}

	if jobID.Valid {
		build.JobID = int(jobID.Int64)
		build.JobName = jobName.String
		build.PipelineName = pipelineName.String
	}

	return build, nil
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}

func buildEventSeq(buildID int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildID)
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

type conditionNotifier struct {
	cond func() (bool, error)

	bus     *notificationsBus
	channel string

	notified chan bool
	notify   chan struct{}

	stop chan struct{}
}

func (notifier *conditionNotifier) Notify() <-chan struct{} {
	return notifier.notify
}

func (notifier *conditionNotifier) Close() error {
	close(notifier.stop)
	return notifier.bus.Unlisten(notifier.channel, notifier.notified)
}

func (notifier *conditionNotifier) watch() {
	for {
		c, err := notifier.cond()
		if err != nil {
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-notifier.stop:
				return
			}
		}

		if c {
			notifier.sendNotification()
		}

	dance:
		for {
			select {
			case <-notifier.stop:
				return
			case ok := <-notifier.notified:
				if ok {
					notifier.sendNotification()
				} else {
					break dance
				}
			}
		}
	}
}

func (notifier *conditionNotifier) sendNotification() {
	select {
	case notifier.notify <- struct{}{}:
	default:
	}
}
