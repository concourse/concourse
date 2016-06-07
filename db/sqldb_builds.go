package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/lib/pq"
)

const buildColumns = "id, name, job_id, team_id, status, scheduled, inputs_determined, engine, engine_metadata, start_time, end_time, reap_time"
const qualifiedBuildColumns = "b.id, b.name, b.job_id, b.team_id, b.status, b.scheduled, b.inputs_determined, b.engine, b.engine_metadata, b.start_time, b.end_time, b.reap_time, j.name as job_name, p.id as pipeline_id, p.name as pipeline_name, t.name as team_name"

func (db *SQLDB) GetAllStartedBuilds() ([]Build, error) {
	rows, err := db.conn.Query(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		LEFT OUTER JOIN jobs j ON b.job_id = j.id
		LEFT OUTER JOIN pipelines p ON j.pipeline_id = p.id
		LEFT OUTER JOIN teams t ON b.team_id = t.id
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
	var teamID int
	var teamName string

	err := row.Scan(&id, &name, &jobID, &teamID, &status, &scheduled, &inputsDetermined, &engine, &engineMetadata, &startTime, &endTime, &reapTime, &jobName, &pipelineID, &pipelineName, &teamName)
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

		TeamID:   teamID,
		TeamName: teamName,
	}

	if jobID.Valid {
		build.JobID = int(jobID.Int64)
		build.JobName = jobName.String
		build.PipelineName = pipelineName.String
		build.PipelineID = int(pipelineID.Int64)
	}

	return build, true, nil
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}
