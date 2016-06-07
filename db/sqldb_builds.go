package db

import (
	"database/sql"
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
