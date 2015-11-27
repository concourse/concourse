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

	conn Conn
	bus  *notificationsBus
}

const buildColumns = "id, name, job_id, status, scheduled, engine, engine_metadata, start_time, end_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_id, b.status, b.scheduled, b.engine, b.engine_metadata, b.start_time, b.end_time, j.name as job_name, p.name as pipeline_name"

func NewSQL(
	logger lager.Logger,
	sqldbConnection Conn,
	bus *notificationsBus,
) *SQLDB {
	return &SQLDB{
		logger: logger,

		conn: sqldbConnection,
		bus:  bus,
	}
}

func (db *SQLDB) InsertVolume(data Volume) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	var resourceVersion []byte

	resourceVersion, err = json.Marshal(data.ResourceVersion)
	if err != nil {
		return err
	}

	interval := fmt.Sprintf("%d second", int(data.TTL.Seconds()))

	defer tx.Rollback()

	_, err = tx.Exec(`
	INSERT INTO volumes(
    worker_name,
		expires_at,
		ttl,
		handle,
		resource_version,
		resource_hash
	) VALUES (
		$1,
		NOW() + $2::INTERVAL,
		$3,
		$4,
		$5,
		$6
	)
	`, data.WorkerName,
		interval,
		data.TTL,
		data.Handle,
		resourceVersion,
		data.ResourceHash,
	)

	if err != nil {
		if strings.Contains(err.Error(), `duplicate key value violates unique constraint "volumes_worker_name_handle_key"`) {
			return nil
		}

		return err
	}

	return tx.Commit()
}

func (db *SQLDB) GetVolumes() ([]SavedVolume, error) {
	// reap expired volumes
	_, err := db.conn.Exec(`
		DELETE FROM volumes
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	if err != nil {
		return nil, err
	}

	rows, err := db.conn.Query(`
		SELECT v.worker_name,
			v.ttl,
			EXTRACT(epoch FROM v.expires_at - NOW()),
			v.handle,
			v.resource_version,
			v.resource_hash,
			v.id
		FROM volumes v
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	volumes := []SavedVolume{}

	for rows.Next() {
		var volume SavedVolume
		var ttlSeconds float64
		var versionJSON []byte

		err := rows.Scan(&volume.WorkerName, &volume.TTL, &ttlSeconds, &volume.Handle, &versionJSON, &volume.ResourceHash, &volume.ID)
		if err != nil {
			return nil, err
		}

		volume.ExpiresIn = time.Duration(ttlSeconds) * time.Second

		err = json.Unmarshal(versionJSON, &volume.ResourceVersion)
		if err != nil {
			return nil, err
		}

		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (db *SQLDB) SetVolumeTTL(volumeData SavedVolume, ttl time.Duration) error {
	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	_, err := db.conn.Exec(`
		UPDATE volumes
		SET expires_at = NOW() + $1::INTERVAL,
		ttl = $2
		WHERE id = $3
	`, interval, ttl, volumeData.ID)

	return err
}

func (db *SQLDB) GetVolumeTTL(handle string) (time.Duration, error) {
	var ttl time.Duration

	err := db.conn.QueryRow(`
		SELECT ttl
		FROM volumes
		WHERE handle = $1
	`, handle).Scan(&ttl)

	return ttl, err
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

func (db *SQLDB) LeaseBuildTracking(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
					AND now() - last_tracked > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_tracked = now()
				WHERE id = $1
			`, buildID)
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

func (db *SQLDB) LeaseBuildScheduling(buildID int, interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"build_id": buildID,
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
					AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
			`, buildID, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE builds
				SET last_scheduled = now()
				WHERE id = $1
			`, buildID)
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

func (db *SQLDB) LeaseCacheInvalidation(interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: db.conn,
		logger: db.logger.Session("lease", lager.Data{
			"CacheInvalidator": "Scottsboro",
		}),
		attemptSignFunc: func(tx *sql.Tx) (sql.Result, error) {
			_, err := tx.Exec(`
				INSERT INTO cache_invalidator (last_invalidated)
				SELECT 'epoch'
				WHERE NOT EXISTS (SELECT * FROM cache_invalidator)`)
			if err != nil {
				return nil, err
			}
			return tx.Exec(`
		  	UPDATE cache_invalidator
				SET last_invalidated = now()
				WHERE now() - last_invalidated > ($1 || ' SECONDS')::INTERVAL
			`, interval.Seconds())
		},
		heartbeatFunc: func(tx *sql.Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE cache_invalidator
				SET last_invalidated = now()
			`)
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
		build, _, err := scanBuild(rows)
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
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (db *SQLDB) GetBuild(buildID int) (Build, bool, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.id = $1
	`, buildID))
}

func (db *SQLDB) getPipelineName(buildID int) (string, error) {
	var pipelineName string
	err := db.conn.QueryRow(`
		SELECT p.name
		FROM builds b, jobs j, pipelines p
		WHERE b.id = $1
		AND b.job_id = j.id
		AND j.pipeline_id = p.id
		LIMIT 1
	`, buildID).Scan(&pipelineName)

	if err != nil {
		return "", err
	}

	return pipelineName, nil
}

func (db *SQLDB) GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	rows, err := db.conn.Query(`
		SELECT i.name, r.name, v.type, v.version, v.metadata,
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
	`, buildID)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	pipelineName, err := db.getPipelineName(buildID)
	if err != nil {
		return nil, nil, err
	}

	for rows.Next() {
		var inputName string
		var vr VersionedResource
		var firstOccurrence bool

		var version, metadata string
		err := rows.Scan(&inputName, &vr.Resource, &vr.Type, &version, &metadata, &firstOccurrence)
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

		vr.PipelineName = pipelineName

		inputs = append(inputs, BuildInput{
			Name:              inputName,
			VersionedResource: vr,
			FirstOccurrence:   firstOccurrence,
		})
	}

	rows, err = db.conn.Query(`
		SELECT r.name, v.type, v.version, v.metadata
		FROM versioned_resources v, build_outputs o, builds b, resources r
		WHERE b.id = $1
		AND o.build_id = b.id
		AND o.versioned_resource_id = v.id
    AND r.id = v.resource_id
		AND o.explicit
	`, buildID)
	if err != nil {
		return nil, nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var vr VersionedResource

		var version, metadata string
		err := rows.Scan(&vr.Resource, &vr.Type, &version, &metadata)
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

		vr.PipelineName = pipelineName

		outputs = append(outputs, BuildOutput{
			VersionedResource: vr,
		})
	}

	return inputs, outputs, nil
}

func (db *SQLDB) getBuildVersionedResouces(buildID int, resourceRequest string) (SavedVersionedResources, error) {

	rows, err := db.conn.Query(resourceRequest, buildID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedVersionedResources := SavedVersionedResources{}

	for rows.Next() {
		var versionedResource SavedVersionedResource
		var versionJSON []byte
		var metadataJSON []byte
		err = rows.Scan(&versionedResource.ID, &versionedResource.Enabled, &versionJSON, &metadataJSON, &versionedResource.Type, &versionedResource.Resource, &versionedResource.PipelineName)

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

func (db *SQLDB) GetBuildInputVersionedResouces(buildID int) (SavedVersionedResources, error) {
	return db.getBuildVersionedResouces(buildID, `
		SELECT vr.id,
			vr.enabled,
			vr.version,
			vr.metadata,
			vr.type,
			r.name,
			p.name
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN build_inputs bi ON bi.build_id = b.id
		INNER JOIN versioned_resources vr ON bi.versioned_resource_id = vr.id
		INNER JOIN resources r ON vr.resource_id = r.id
		WHERE b.id = $1`)
}

func (db *SQLDB) GetBuildOutputVersionedResouces(buildID int) (SavedVersionedResources, error) {
	return db.getBuildVersionedResouces(buildID, `
		SELECT vr.id,
			vr.enabled,
			vr.version,
			vr.metadata,
			vr.type,
			r.name,
			p.name
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN build_outputs bo ON bo.build_id = b.id
		INNER JOIN versioned_resources vr ON bo.versioned_resource_id = vr.id
		INNER JOIN resources r ON vr.resource_id = r.id
		WHERE b.id = $1 AND bo.explicit`)
}

func (db *SQLDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	build, _, err := scanBuild(tx.QueryRow(`
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

	err = db.bus.Notify(buildEventsChannel(buildID))
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

	err = db.bus.Notify(buildEventsChannel(buildID))
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

func (db *SQLDB) SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	pipelineDBFactory := NewPipelineDBFactory(db.logger, db.conn, db.bus, db)
	pipelineDB, err := pipelineDBFactory.BuildWithName(vr.PipelineName)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return pipelineDB.SaveBuildOutput(buildID, vr, explicit)
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

	err = db.bus.Notify(buildAbortChannel(buildID))
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

	err = db.bus.Notify(buildEventsChannel(buildID))
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
			SET addr = $1, expires = NULL, active_containers = $2, resource_types = $3, platform = $4, tags = $5, baggageclaim_url = $6, name = $7
			WHERE name = $7 OR addr = $1
		`, info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected == 0 {
			_, err := db.conn.Exec(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NULL, $2, $3, $4, $5, $6, $7)
			`, info.GardenAddr, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
			if err != nil {
				return err
			}
		}

		return nil
	} else {
		interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

		result, err := db.conn.Exec(`
			UPDATE workers
			SET addr = $1, expires = NOW() + $2::INTERVAL, active_containers = $3, resource_types = $4, platform = $5, tags = $6, baggageclaim_url = $7, name = $8
			WHERE name = $8 OR addr = $1
		`, info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
		if err != nil {
			return err
		}

		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}

		if affected == 0 {
			_, err := db.conn.Exec(`
				INSERT INTO workers (addr, expires, active_containers, resource_types, platform, tags, baggageclaim_url, name)
				VALUES ($1, NOW() + $2::INTERVAL, $3, $4, $5, $6, $7, $8)
			`, info.GardenAddr, interval, info.ActiveContainers, resourceTypes, info.Platform, tags, info.BaggageclaimURL, info.Name)
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

	// TODO: Clean this up after people have upgraded and we can guarantee the name field is present and populated
	// select remaining workers
	rows, err := db.conn.Query(`
		SELECT addr, active_containers, resource_types, platform, tags, baggageclaim_url,
			CASE
				WHEN COALESCE(name, '') = '' then addr
				ELSE name
			END as name
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

		err := rows.Scan(&info.GardenAddr, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.BaggageclaimURL, &info.Name)
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

func (db *SQLDB) GetWorker(name string) (WorkerInfo, bool, error) {
	// reap expired workers
	_, err := db.conn.Exec(`
		DELETE FROM workers
		WHERE expires IS NOT NULL
		AND expires < NOW()
	`)
	if err != nil {
		return WorkerInfo{}, false, err
	}

	var info WorkerInfo
	var resourceTypes []byte
	var tags []byte

	// TODO: Clean this up after people have upgraded and we can guarantee the name field is present and populated
	err = db.conn.QueryRow(`
		SELECT addr, baggageclaim_url, active_containers, resource_types, platform, tags,
			CASE
				WHEN COALESCE(name, '') = '' then addr
				ELSE name
			END as name
		FROM workers
		WHERE
			CASE
				WHEN COALESCE(name, '') = '' then addr = $1
				ELSE name = $1
			END
	`, name).Scan(&info.GardenAddr, &info.BaggageclaimURL, &info.ActiveContainers, &resourceTypes, &info.Platform, &tags, &info.Name)

	if err != nil {
		if err == sql.ErrNoRows {
			return WorkerInfo{}, false, nil
		}

		return WorkerInfo{}, false, err
	}

	err = json.Unmarshal(resourceTypes, &info.ResourceTypes)
	if err != nil {
		return WorkerInfo{}, false, err
	}

	err = json.Unmarshal(tags, &info.Tags)
	if err != nil {
		return WorkerInfo{}, false, err
	}

	return info, true, nil
}

func (db *SQLDB) DeleteContainer(handle string) error {
	_, err := db.conn.Exec(`
		DELETE FROM containers
		WHERE handle = $1
	`, handle)
	return err
}

func (db *SQLDB) FindContainersByIdentifier(id ContainerIdentifier) ([]Container, error) {
	_, err := db.conn.Exec(`
		DELETE FROM containers
		WHERE expires_at IS NOT NULL
		AND expires_at < NOW()
	`)
	if err != nil {
		return nil, err
	}

	whereCriteria := []string{}
	params := []interface{}{}

	if id.Name != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("name = $%d", len(params)+1))
		params = append(params, id.Name)
	}

	if id.PipelineName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("pipeline_name = $%d", len(params)+1))
		params = append(params, id.PipelineName)
	}

	if id.BuildID != 0 {
		whereCriteria = append(whereCriteria, fmt.Sprintf("build_id = $%d", len(params)+1))
		params = append(params, id.BuildID)
	}

	if id.Type != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("type = $%d", len(params)+1))
		params = append(params, id.Type.String())
	}

	if id.WorkerName != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("worker_name = $%d", len(params)+1))
		params = append(params, id.WorkerName)
	}

	if id.CheckType != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_type = $%d", len(params)+1))
		params = append(params, id.CheckType)
	}

	var checkSourceBlob []byte
	if id.CheckSource != nil {
		checkSourceBlob, err = json.Marshal(id.CheckSource)
		if err != nil {
			return nil, err
		}
		whereCriteria = append(whereCriteria, fmt.Sprintf("check_source = $%d", len(params)+1))
		params = append(params, checkSourceBlob)
	}

	if id.PlanID != "" {
		whereCriteria = append(whereCriteria, fmt.Sprintf("plan_id = $%d", len(params)+1))
		params = append(params, string(id.PlanID))
	}

	var rows *sql.Rows
	selectQuery := `
		SELECT handle, pipeline_name, type, name, build_id, worker_name, expires_at, check_type, check_source, plan_id, working_directory, env_variables
		FROM containers
	`

	if len(whereCriteria) > 0 {
		selectQuery += fmt.Sprintf("WHERE %s", strings.Join(whereCriteria, " AND "))
	}

	rows, err = db.conn.Query(selectQuery, params...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	infos := []Container{}
	for rows.Next() {
		info, err := scanContainer(rows)

		if err != nil {
			return nil, err
		}

		infos = append(infos, info)
	}

	return infos, nil
}

func (db *SQLDB) FindContainerByIdentifier(id ContainerIdentifier) (Container, bool, error) {
	containers, err := db.FindContainersByIdentifier(id)
	if err != nil {
		return Container{}, false, err
	}

	switch len(containers) {
	case 0:
		return Container{}, false, nil

	case 1:
		return containers[0], true, nil

	default:
		return Container{}, false, ErrMultipleContainersFound
	}
}

func (db *SQLDB) GetContainer(handle string) (Container, bool, error) {
	info, err := scanContainer(db.conn.QueryRow(`
		SELECT handle, pipeline_name, type, name, build_id, worker_name, expires_at, check_type, check_source, plan_id, working_directory, env_variables
		FROM containers c
		WHERE c.handle = $1
	`, handle))

	if err != nil {
		if err == sql.ErrNoRows {
			return Container{}, false, nil
		}
		return Container{}, false, err
	}

	return info, true, nil
}

func (db *SQLDB) CreateContainer(container Container, ttl time.Duration) error {
	tx, err := db.conn.Begin()

	if err != nil {
		return err
	}

	checkSource, err := json.Marshal(container.CheckSource)
	if err != nil {
		return err
	}

	envVariables, err := json.Marshal(container.EnvironmentVariables)
	if err != nil {
		return err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO containers (handle, name, pipeline_name, build_id, type, worker_name, expires_at, check_type, check_source, plan_id, working_directory, env_variables)
		VALUES ($1, $2, $3, $4, $5, $6,  NOW() + $7::INTERVAL, $8, $9, $10, $11, $12)
		`,
		container.Handle,
		container.Name,
		container.PipelineName,
		container.BuildID,
		container.Type.String(),
		container.WorkerName,
		interval,
		container.CheckType,
		checkSource,
		string(container.PlanID),
		container.WorkingDirectory,
		envVariables,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) UpdateExpiresAtOnContainer(handle string, ttl time.Duration) error {
	tx, err := db.conn.Begin()

	if err != nil {
		return err
	}

	interval := fmt.Sprintf("%d second", int(ttl.Seconds()))

	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE containers SET expires_at = NOW() + $2::INTERVAL
		WHERE handle = $1
		`,
		handle,
		interval,
	)

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) ReapContainer(handle string) error {
	rows, err := db.conn.Exec(`
		DELETE FROM containers WHERE handle = $1
	`, handle)
	if err != nil {
		return err
	}

	affected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	// just to be explicit: reaping 0 containers is fine;
	// it may have already expired
	if affected == 0 {
		return nil
	}

	return nil
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

func scanContainer(row scannable) (Container, error) {
	var infoType string
	var checkSourceBlob []byte
	var envVariablesBlob []byte

	info := Container{}

	var planID sql.NullString
	err := row.Scan(&info.Handle, &info.PipelineName, &infoType, &info.Name, &info.BuildID, &info.WorkerName, &info.ExpiresAt, &info.CheckType, &checkSourceBlob, &planID, &info.WorkingDirectory, &envVariablesBlob)
	if err != nil {
		return Container{}, err
	}

	info.Type, err = containerTypeFromString(infoType)
	if err != nil {
		return Container{}, err
	}

	err = json.Unmarshal(checkSourceBlob, &info.CheckSource)
	if err != nil {
		return Container{}, err
	}

	info.PlanID = atc.PlanID(planID.String)

	err = json.Unmarshal(envVariablesBlob, &info.EnvironmentVariables)
	if err != nil {
		return Container{}, err
	}

	return info, nil
}

func scanBuild(row scannable) (Build, bool, error) {
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
			return Build{}, false, nil
		}

		return Build{}, false, err
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

	return build, true, nil
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
