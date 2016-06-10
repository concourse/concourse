package db

import (
	"database/sql"

	"github.com/lib/pq"
)

func newBuildFactory(conn Conn, bus *notificationsBus) *buildFactory {
	return &buildFactory{
		conn: conn,
		bus:  bus,
	}
}

type buildFactory struct {
	conn Conn
	bus  *notificationsBus
}

func (f *buildFactory) ScanBuild(row scannable) (BuildDB, bool, error) {
	var id int
	var name string
	var jobID, pipelineID sql.NullInt64
	var status string
	var scheduled bool
	var engine, engineMetadata, jobName, pipelineName sql.NullString
	var startTime pq.NullTime
	var endTime pq.NullTime
	var reapTime pq.NullTime
	var teamName string

	err := row.Scan(&id, &name, &jobID, &status, &scheduled, &engine, &engineMetadata, &startTime, &endTime, &reapTime, &jobName, &pipelineID, &pipelineName, &teamName)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}

		return nil, false, err
	}

	build := SavedBuild{
		id:        id,
		Name:      name,
		Status:    Status(status),
		Scheduled: scheduled,

		Engine:         engine.String,
		EngineMetadata: engineMetadata.String,

		StartTime: startTime.Time,
		EndTime:   endTime.Time,
		ReapTime:  reapTime.Time,

		TeamName: teamName,
	}

	if jobID.Valid {
		build.JobName = jobName.String
		build.PipelineName = pipelineName.String
		build.PipelineID = int(pipelineID.Int64)
	}

	return &buildDB{
		build: build,
		conn:  f.conn,
		bus:   f.bus,
	}, true, nil
}
