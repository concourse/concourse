package dbng

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
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

	Interceptible() (bool, error)

	SaveStatus(s BuildStatus) error
	SaveImageResourceVersion(planID atc.PlanID, resourceVersion atc.Version, resourceHash string) error
	SetInterceptible(bool) error

	Finish(s BuildStatus) error
	Delete() (bool, error)
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

	conn Conn
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

	err = tx.QueryRow(`
		UPDATE builds
		SET status = $2, end_time = now(), completed = true
		WHERE id = $1
		RETURNING end_time
	`, b.id, string(s)).Scan(&endTime)
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
