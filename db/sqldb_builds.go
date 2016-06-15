package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/event"
	"github.com/lib/pq"
)

const buildColumns = "id, name, job_id, status, scheduled, inputs_determined, engine, engine_metadata, start_time, end_time, reap_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_id, b.status, b.scheduled, b.inputs_determined, b.engine, b.engine_metadata, b.start_time, b.end_time, b.reap_time, j.name as job_name, p.id as pipeline_id, p.name as pipeline_name"

func (db *SQLDB) GetBuilds(page Page) ([]Build, Pagination, error) {
	query := `
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
	`

	var rows *sql.Rows
	var err error

	if page.Since == 0 && page.Until == 0 {
		rows, err = db.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY b.id DESC
			LIMIT $1
		`, query), page.Limit)
	} else if page.Until != 0 {
		rows, err = db.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
				WHERE b.id > $1
				ORDER BY b.id ASC
				LIMIT $2
			) sub
			ORDER BY sub.id DESC
		`, query), page.Until, page.Limit)
	} else {
		rows, err = db.conn.Query(fmt.Sprintf(`
			%s
			WHERE b.id < $1
			ORDER BY b.id DESC
			LIMIT $2
		`, query), page.Since, page.Limit)
	}

	if err != nil {
		return nil, Pagination{}, err
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return builds, Pagination{}, nil
	}

	var minID int
	var maxID int

	err = db.conn.QueryRow(`
		SELECT COALESCE(MAX(b.id), 0) as maxID,
			COALESCE(MIN(b.id), 0) as minID
		FROM builds b
	`).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	first := builds[0]
	last := builds[len(builds)-1]

	var pagination Pagination

	if first.ID < maxID {
		pagination.Previous = &Page{
			Until: first.ID,
			Limit: page.Limit,
		}
	}

	if last.ID > minID {
		pagination.Next = &Page{
			Since: last.ID,
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
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

func (db *SQLDB) GetLatestFinishedBuild(jobID int) (Build, bool, error) {
	return scanBuild(db.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.completed = true
		AND b.job_id = $1
		ORDER BY b.end_time DESC
		LIMIT 1
		`, jobID))
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
	`, buildID)
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

	rows, err = db.conn.Query(`
		SELECT r.name, v.type, v.version, v.metadata, r.pipeline_id
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

func (db *SQLDB) getBuildVersionedResources(buildID int, resourceRequest string) (SavedVersionedResources, error) {

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

func (db *SQLDB) GetBuildVersionedResources(buildID int) (SavedVersionedResources, error) {
	return db.getBuildVersionedResources(buildID, `
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

func (db *SQLDB) CreateOneOffBuild() (Build, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	build, _, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, status)
		VALUES (nextval('one_off_name'), 'pending')
		RETURNING ` + buildColumns + `, null, null, null
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

	err = db.buildPrepHelper.CreateBuildPreparation(tx, build.ID)
	if err != nil {
		return Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (db *SQLDB) GetBuildPreparation(passedBuildID int) (BuildPreparation, bool, error) {
	return db.buildPrepHelper.GetBuildPreparation(db.conn, passedBuildID)
}

func (db *SQLDB) UpdateBuildPreparation(buildPrep BuildPreparation) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = db.buildPrepHelper.UpdateBuildPreparation(tx, buildPrep)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (db *SQLDB) ResetBuildPreparationsWithPipelinePaused(pipelineID int) error {
	_, err := db.conn.Exec(`
			UPDATE build_preparation
			SET paused_pipeline='blocking',
			    paused_job='unknown',
					max_running_builds='unknown',
					inputs='{}',
					inputs_satisfied='unknown'
			FROM build_preparation bp, builds b, jobs j
			WHERE bp.build_id = b.id AND b.job_id = j.id
				AND j.pipeline_id = $1 AND b.status = 'pending' AND b.scheduled = false
		`, pipelineID)
	return err
}

func (db *SQLDB) StartBuild(buildID int, pipelineID int, engine, metadata string) (bool, error) {
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

	err = db.saveBuildEvent(tx, buildID, pipelineID, event.Status{
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

func (db *SQLDB) FinishBuild(buildID int, pipelineID int, status Status) error {
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

	err = db.saveBuildEvent(tx, buildID, pipelineID, event.Status{
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

func (db *SQLDB) ErrorBuild(buildID int, pipelineID int, cause error) error {
	err := db.SaveBuildEvent(buildID, pipelineID, event.Error{
		Message: cause.Error(),
	})
	if err != nil {
		return err
	}

	return db.FinishBuild(buildID, pipelineID, StatusErrored)
}

func (db *SQLDB) SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error) {
	pipelineDBFactory := NewPipelineDBFactory(db.conn, db.bus, db)
	pipelineDB, err := pipelineDBFactory.BuildWithID(input.VersionedResource.PipelineID)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return pipelineDB.SaveBuildInput(buildID, input)
}

func (db *SQLDB) SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	pipelineDBFactory := NewPipelineDBFactory(db.conn, db.bus, db)
	pipelineDB, err := pipelineDBFactory.BuildWithID(vr.PipelineID)
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

	build, _, err := db.GetBuild(buildID)
	if err != nil {
		return nil, err
	}

	table := "build_events"
	if build.PipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", build.PipelineID)
	}

	return newSQLDBBuildEventSource(
		buildID,
		table,
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

func (db *SQLDB) DeleteBuildEventsByBuildIDs(buildIDs []int) error {
	if len(buildIDs) == 0 {
		return nil
	}

	interfaceBuildIDs := make([]interface{}, len(buildIDs))
	for i, buildID := range buildIDs {
		interfaceBuildIDs[i] = buildID
	}

	indexStrings := make([]string, len(buildIDs))
	for i := range indexStrings {
		indexStrings[i] = "$" + strconv.Itoa(i+1)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(`
   DELETE FROM build_events
	 WHERE build_id IN (`+strings.Join(indexStrings, ",")+`)
	 `, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET reap_time = now()
		WHERE id IN (`+strings.Join(indexStrings, ",")+`)
	`, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (db *SQLDB) SaveBuildEvent(buildID int, pipelineID int, event atc.Event) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = db.saveBuildEvent(tx, buildID, pipelineID, event)
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

func (db *SQLDB) saveBuildEvent(tx Tx, buildID int, pipelineID int, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	table := "build_events"
	if pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", pipelineID)
	}

	_, err = tx.Exec(fmt.Sprintf(`
		INSERT INTO %s (event_id, build_id, type, version, payload)
		VALUES (nextval('%s'), $1, $2, $3, $4)
	`, table, buildEventSeq(buildID)), buildID, string(event.EventType()), string(event.Version()), payload)
	if err != nil {
		return err
	}

	return nil
}

func scanBuild(row scannable) (Build, bool, error) {
	var id int
	var name string
	var jobID, pipelineID sql.NullInt64
	var status string
	var scheduled bool
	var inputsDetermined bool
	var engine, engineMetadata, jobName, pipelineName sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime
	var reapTime pq.NullTime

	err := row.Scan(&id, &name, &jobID, &status, &scheduled, &inputsDetermined, &engine, &engineMetadata, &startTime, &endTime, &reapTime, &jobName, &pipelineID, &pipelineName)
	if err != nil {
		if err == sql.ErrNoRows {
			return Build{}, false, nil
		}

		return Build{}, false, err
	}

	build := Build{
		ID:               id,
		Name:             name,
		Status:           Status(status),
		Scheduled:        scheduled,
		InputsDetermined: inputsDetermined,

		Engine:         engine.String,
		EngineMetadata: engineMetadata.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
		ReapTime:  reapTime.Time,
	}

	if jobID.Valid {
		build.JobID = int(jobID.Int64)
		build.JobName = jobName.String
		build.PipelineName = pipelineName.String
		build.PipelineID = int(pipelineID.Int64)
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
