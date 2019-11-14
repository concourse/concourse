package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/lib/pq"
)

type InputVersionEmptyError struct {
	InputName string
}

func (e InputVersionEmptyError) Error() string {
	return fmt.Sprintf("input '%s' has successfully resolved but contains missing version information", e.InputName)
}

//go:generate counterfeiter . Job

type Job interface {
	ID() int
	Name() string
	Paused() bool
	FirstLoggedBuildID() int
	PipelineID() int
	PipelineName() string
	TeamID() int
	TeamName() string
	Config() atc.JobConfig
	Tags() []string
	Public() bool
	ScheduleRequested() time.Time

	Reload() (bool, error)

	Pause() error
	Unpause() error

	ScheduleBuild(Build) (bool, error)
	CreateBuild() (Build, error)
	RerunBuild(Build) (Build, error)

	RequestSchedule() error

	Builds(page Page) ([]Build, Pagination, error)
	BuildsWithTime(page Page) ([]Build, Pagination, error)
	Build(name string) (Build, bool, error)
	FinishedAndNextBuild() (Build, Build, error)
	UpdateFirstLoggedBuildID(newFirstLoggedBuildID int) error
	EnsurePendingBuildExists() error
	GetPendingBuilds() ([]Build, error)

	GetNextBuildInputs() ([]BuildInput, error)
	GetFullNextBuildInputs() ([]BuildInput, bool, error)
	SaveNextInputMapping(inputMapping InputMapping, inputsDetermined bool) error

	ClearTaskCache(string, string) (int64, error)

	AcquireSchedulingLock(lager.Logger) (lock.Lock, bool, error)

	SetHasNewInputs(bool) error
	HasNewInputs() bool
}

var jobsQuery = psql.Select("j.id", "j.name", "j.config", "j.paused", "j.first_logged_build_id", "j.pipeline_id", "p.name", "p.team_id", "t.name", "j.nonce", "j.tags", "j.has_new_inputs", "j.schedule_requested").
	From("jobs j, pipelines p").
	LeftJoin("teams t ON p.team_id = t.id").
	Where(sq.Expr("j.pipeline_id = p.id"))

type FirstLoggedBuildIDDecreasedError struct {
	Job   string
	OldID int
	NewID int
}

func (e FirstLoggedBuildIDDecreasedError) Error() string {
	return fmt.Sprintf("first logged build id for job '%s' decreased from %d to %d", e.Job, e.OldID, e.NewID)
}

type job struct {
	id                 int
	name               string
	paused             bool
	firstLoggedBuildID int
	pipelineID         int
	pipelineName       string
	teamID             int
	teamName           string
	config             atc.JobConfig
	tags               []string
	hasNewInputs       bool
	scheduleRequested  time.Time

	conn        Conn
	lockFactory lock.LockFactory
}

func (j *job) SetHasNewInputs(hasNewInputs bool) error {
	result, err := psql.Update("jobs").
		Set("has_new_inputs", hasNewInputs).
		Where(sq.Eq{"id": j.id}).
		RunWith(j.conn).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	return nil
}

type Jobs []Job

func (jobs Jobs) Configs() atc.JobConfigs {
	var configs atc.JobConfigs

	for _, j := range jobs {
		configs = append(configs, j.Config())
	}

	return configs
}

func (j *job) ID() int                      { return j.id }
func (j *job) Name() string                 { return j.name }
func (j *job) Paused() bool                 { return j.paused }
func (j *job) FirstLoggedBuildID() int      { return j.firstLoggedBuildID }
func (j *job) PipelineID() int              { return j.pipelineID }
func (j *job) PipelineName() string         { return j.pipelineName }
func (j *job) TeamID() int                  { return j.teamID }
func (j *job) TeamName() string             { return j.teamName }
func (j *job) Config() atc.JobConfig        { return j.config }
func (j *job) Tags() []string               { return j.tags }
func (j *job) Public() bool                 { return j.Config().Public }
func (j *job) HasNewInputs() bool           { return j.hasNewInputs }
func (j *job) ScheduleRequested() time.Time { return j.scheduleRequested }

func (j *job) Reload() (bool, error) {
	row := jobsQuery.Where(sq.Eq{"j.id": j.id}).
		RunWith(j.conn).
		QueryRow()

	err := scanJob(j, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (j *job) Pause() error {
	return j.updatePausedJob(true)
}

func (j *job) Unpause() error {
	return j.updatePausedJob(false)
}

func (j *job) FinishedAndNextBuild() (Build, Build, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer Rollback(tx)

	next, err := j.nextBuild(tx)
	if err != nil {
		return nil, nil, err
	}

	finished, err := j.finishedBuild(tx)
	if err != nil {
		return nil, nil, err
	}

	// query next build again if the build state changed between the two queries
	if next != nil && finished != nil && next.ID() == finished.ID() {
		next = nil

		next, err = j.nextBuild(tx)
		if err != nil {
			return nil, nil, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	return finished, next, nil
}

func (j *job) UpdateFirstLoggedBuildID(newFirstLoggedBuildID int) error {
	if j.firstLoggedBuildID > newFirstLoggedBuildID {
		return FirstLoggedBuildIDDecreasedError{
			Job:   j.name,
			OldID: j.firstLoggedBuildID,
			NewID: newFirstLoggedBuildID,
		}
	}

	result, err := psql.Update("jobs").
		Set("first_logged_build_id", newFirstLoggedBuildID).
		Where(sq.Eq{"id": j.id}).
		RunWith(j.conn).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) BuildsWithTime(page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.Where(sq.Eq{"j.id": j.id})
	newMinMaxIdQuery := minMaxIdQuery.
		Join("jobs j ON b.job_id = j.id").
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		})
	return getBuildsWithDates(newBuildsQuery, newMinMaxIdQuery, page, j.conn, j.lockFactory)
}

func (j *job) Builds(page Page) ([]Build, Pagination, error) {
	newBuildsQuery := buildsQuery.Where(sq.Eq{"j.id": j.id})
	newMinMaxIdQuery := minMaxIdQuery.
		Join("jobs j ON b.job_id = j.id").
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		})

	return getBuildsWithPagination(newBuildsQuery, newMinMaxIdQuery, page, j.conn, j.lockFactory)
}

func (j *job) Build(name string) (Build, bool, error) {
	var query sq.SelectBuilder

	if name == "latest" {
		query = buildsQuery.
			Where(sq.Eq{"b.job_id": j.id}).
			OrderBy("b.id DESC").
			Limit(1)
	} else {
		query = buildsQuery.Where(sq.Eq{
			"b.job_id": j.id,
			"b.name":   name,
		})
	}

	row := query.RunWith(j.conn).QueryRow()

	build := &build{conn: j.conn, lockFactory: j.lockFactory}

	err := scanBuild(build, row, j.conn.EncryptionStrategy())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return build, true, nil
}

func (j *job) ScheduleBuild(build Build) (bool, error) {
	if build.IsScheduled() {
		return true, nil
	}

	tx, err := j.conn.Begin()
	if err != nil {
		return false, err
	}

	defer tx.Rollback()

	reached, err := j.isMaxInFlightReached(tx, build.ID())
	if err != nil {
		return false, err
	}

	result, err := psql.Update("jobs").
		Set("max_in_flight_reached", reached).
		Where(sq.Eq{
			"id": j.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	if rowsAffected != 1 {
		return false, NonOneRowAffectedError{rowsAffected}
	}

	var scheduled bool
	if !reached {
		result, err = psql.Update("builds").
			Set("scheduled", true).
			Where(sq.Eq{"id": build.ID()}).
			RunWith(tx).
			Exec()
		if err != nil {
			return false, err
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return false, err
		}

		if rowsAffected != 1 {
			return false, NonOneRowAffectedError{rowsAffected}
		}

		scheduled = true
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return scheduled, nil
}

func (j *job) GetFullNextBuildInputs() ([]BuildInput, bool, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var inputsDetermined bool
	err = psql.Select("inputs_determined").
		From("jobs").
		Where(sq.Eq{
			"name":        j.name,
			"pipeline_id": j.pipelineID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&inputsDetermined)
	if err != nil {
		return nil, false, err
	}

	if !inputsDetermined {
		return nil, false, nil
	}

	buildInputs, err := j.getNextBuildInputs(tx)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return buildInputs, true, nil
}

func (j *job) GetNextBuildInputs() ([]BuildInput, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	buildInputs, err := j.getNextBuildInputs(tx)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return buildInputs, nil
}

func (j *job) EnsurePendingBuildExists() error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	buildName, err := j.getNewBuildName(tx)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		INSERT INTO builds (name, job_id, pipeline_id, team_id, status)
		SELECT $1, $2, $3, $4, 'pending'
		WHERE NOT EXISTS
			(SELECT id FROM builds WHERE job_id = $2 AND status = 'pending')
		RETURNING id
	`, buildName, j.id, j.pipelineID, j.teamID)
	if err != nil {
		return err
	}

	defer Close(rows)

	if rows.Next() {
		var buildID int
		err := rows.Scan(&buildID)
		if err != nil {
			return err
		}

		err = rows.Close()
		if err != nil {
			return err
		}

		err = createBuildEventSeq(tx, buildID)
		if err != nil {
			return err
		}

		return tx.Commit()
	}

	return nil
}

func (j *job) GetPendingBuilds() ([]Build, error) {
	builds := []Build{}

	row := jobsQuery.Where(sq.Eq{
		"j.name":        j.name,
		"j.active":      true,
		"j.pipeline_id": j.pipelineID,
	}).RunWith(j.conn).QueryRow()

	job := &job{conn: j.conn, lockFactory: j.lockFactory}
	err := scanJob(job, row)
	if err != nil {
		return nil, err
	}

	rows, err := buildsQuery.
		Where(sq.Eq{
			"b.job_id": j.id,
			"b.status": BuildStatusPending,
		}).
		OrderBy("b.id ASC").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		build := &build{conn: j.conn, lockFactory: j.lockFactory}
		err = scanBuild(build, rows, j.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, nil
}

func (j *job) CreateBuild() (Build, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	buildName, err := j.getNewBuildName(tx)
	if err != nil {
		return nil, err
	}

	build := &build{conn: j.conn, lockFactory: j.lockFactory}
	err = createBuild(tx, build, map[string]interface{}{
		"name":               buildName,
		"job_id":             j.id,
		"pipeline_id":        j.pipelineID,
		"team_id":            j.teamID,
		"status":             BuildStatusPending,
		"manually_triggered": true,
	})
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = requestSchedule(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (j *job) RerunBuild(buildToRerun Build) (Build, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	buildToRerunID := buildToRerun.ID()
	if buildToRerun.RerunOf() != 0 {
		buildToRerunID = buildToRerun.RerunOf()
	}

	rerunBuildName, rerunNumber, err := j.getNewRerunBuildName(tx, buildToRerunID)
	if err != nil {
		return nil, err
	}

	rerunBuild := &build{conn: j.conn, lockFactory: j.lockFactory}
	err = createBuild(tx, rerunBuild, map[string]interface{}{
		"name":         rerunBuildName,
		"job_id":       j.id,
		"pipeline_id":  j.pipelineID,
		"team_id":      j.teamID,
		"status":       BuildStatusPending,
		"rerun_of":     buildToRerunID,
		"rerun_number": rerunNumber,
	})
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = requestSchedule(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return rerunBuild, nil
}

func (j *job) ClearTaskCache(stepName string, cachePath string) (int64, error) {
	tx, err := j.conn.Begin()
	if err != nil {
		return 0, err
	}

	defer Rollback(tx)

	var sqlBuilder sq.DeleteBuilder = psql.Delete("task_caches").
		Where(sq.Eq{
			"job_id":    j.id,
			"step_name": stepName,
		})

	if len(cachePath) > 0 {
		sqlBuilder = sqlBuilder.Where(sq.Eq{"path": cachePath})
	}

	sqlResult, err := sqlBuilder.
		RunWith(tx).
		Exec()

	if err != nil {
		return 0, err
	}

	rowsDeleted, err := sqlResult.RowsAffected()

	if err != nil {
		return 0, err
	}

	return rowsDeleted, tx.Commit()
}

func (j *job) AcquireSchedulingLock(logger lager.Logger) (lock.Lock, bool, error) {
	return j.lockFactory.Acquire(
		logger.Session("lock", lager.Data{
			"job":      j.name,
			"pipeline": j.pipelineName,
		}),
		lock.NewJobSchedulingLockID(j.id),
	)
}

func (j *job) isMaxInFlightReached(tx Tx, buildID int) (bool, error) {
	maxInFlight := j.config.MaxInFlight()

	if maxInFlight == 0 {
		return false, nil
	}

	builds, err := j.getRunningBuildsBySerialGroup(tx)
	if err != nil {
		return false, err
	}

	if len(builds) >= maxInFlight {
		return true, nil
	}

	nextMostPendingBuild, found, err := j.getNextPendingBuildBySerialGroup(tx)
	if err != nil {
		return false, err
	}

	if !found {
		return true, nil
	}

	return nextMostPendingBuild.ID() != buildID, nil
}

func (j *job) RequestSchedule() error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = requestSchedule(tx, j.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (j *job) getRunningBuildsBySerialGroup(tx Tx) ([]Build, error) {
	serialGroups := j.config.GetSerialGroups()
	err := j.updateSerialGroups(tx, serialGroups)
	if err != nil {
		return nil, err
	}

	rows, err := buildsQuery.Options(`DISTINCT ON (b.id)`).
		Join(`jobs_serial_groups jsg ON j.id = jsg.job_id`).
		Where(sq.Eq{
			"jsg.serial_group": serialGroups,
			"j.pipeline_id":    j.pipelineID,
		}).
		Where(sq.Eq{"b.completed": false, "b.scheduled": true}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	bs := []Build{}

	for rows.Next() {
		build := &build{conn: j.conn, lockFactory: j.lockFactory}
		err = scanBuild(build, rows, j.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (j *job) getNextPendingBuildBySerialGroup(tx Tx) (Build, bool, error) {
	serialGroups := j.config.GetSerialGroups()
	err := j.updateSerialGroups(tx, serialGroups)
	if err != nil {
		return nil, false, err
	}

	row := buildsQuery.Options(`DISTINCT ON (b.id)`).
		Join(`jobs_serial_groups jsg ON j.id = jsg.job_id`).
		Where(sq.Eq{
			"jsg.serial_group":    serialGroups,
			"b.status":            BuildStatusPending,
			"j.paused":            false,
			"j.inputs_determined": true,
			"j.pipeline_id":       j.pipelineID}).
		OrderBy("b.id ASC").
		Limit(1).
		RunWith(tx).
		QueryRow()

	build := &build{conn: j.conn, lockFactory: j.lockFactory}
	err = scanBuild(build, row, j.conn.EncryptionStrategy())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return build, true, nil
}

func (j *job) updateSerialGroups(tx Tx, serialGroups []string) error {
	_, err := psql.Delete("jobs_serial_groups").
		Where(sq.Eq{
			"job_id": j.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	for _, serialGroup := range serialGroups {
		_, err = psql.Insert("jobs_serial_groups (job_id, serial_group)").
			Values(j.id, serialGroup).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}

func (j *job) updatePausedJob(pause bool) error {
	result, err := psql.Update("jobs").
		Set("paused", pause).
		Where(sq.Eq{"id": j.id}).
		RunWith(j.conn).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) getNewBuildName(tx Tx) (string, error) {
	var buildName string
	err := psql.Update("jobs").
		Set("build_number_seq", sq.Expr("build_number_seq + 1")).
		Where(sq.Eq{
			"name":        j.name,
			"pipeline_id": j.pipelineID,
		}).
		Suffix("RETURNING build_number_seq").
		RunWith(tx).
		QueryRow().
		Scan(&buildName)

	return buildName, err
}

func (j *job) SaveNextInputMapping(inputMapping InputMapping, inputsDetermined bool) error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Update("jobs").
		Set("inputs_determined", inputsDetermined).
		Where(sq.Eq{"id": j.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	_, err = psql.Delete("next_build_inputs").
		Where(sq.Eq{"job_id": j.id}).
		RunWith(tx).Exec()
	if err != nil {
		return err
	}

	builder := psql.Insert("next_build_inputs").
		Columns("input_name", "job_id", "version_md5", "resource_id", "first_occurrence", "resolve_error")

	for inputName, inputResult := range inputMapping {
		var resolveError sql.NullString
		var firstOccurrence sql.NullBool
		var versionMD5 sql.NullString
		var resourceID sql.NullInt64

		if inputResult.ResolveError != "" {
			resolveError = sql.NullString{String: string(inputResult.ResolveError), Valid: true}
		} else {
			if inputResult.Input == nil {
				return InputVersionEmptyError{inputName}
			}

			firstOccurrence = sql.NullBool{Bool: inputResult.Input.FirstOccurrence, Valid: true}
			resourceID = sql.NullInt64{Int64: int64(inputResult.Input.ResourceID), Valid: true}
			versionMD5 = sql.NullString{String: string(inputResult.Input.Version), Valid: true}
		}

		builder = builder.Values(inputName, j.id, versionMD5, resourceID, firstOccurrence, resolveError)
	}

	if len(inputMapping) != 0 {
		_, err = builder.RunWith(tx).Exec()
		if err != nil {
			return err
		}
	}

	_, err = psql.Delete("next_build_pipes").
		Where(sq.Eq{"to_job_id": j.id}).
		RunWith(tx).Exec()
	if err != nil {
		return err
	}

	pipesBuilder := psql.Insert("next_build_pipes").
		Columns("to_job_id", "from_build_id")

	insertPipes := false
	for _, inputVersion := range inputMapping {
		for _, buildID := range inputVersion.PassedBuildIDs {
			pipesBuilder = pipesBuilder.Values(j.ID(), buildID)
			insertPipes = true
		}
	}

	if insertPipes {
		_, err = pipesBuilder.Suffix("ON CONFLICT DO NOTHING").RunWith(tx).Exec()
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (j *job) nextBuild(tx Tx) (Build, error) {
	var next Build

	row := buildsQuery.
		Where(sq.Eq{"j.id": j.id}).
		Where(sq.Expr("b.id = j.next_build_id")).
		RunWith(tx).
		QueryRow()

	nextBuild := &build{conn: j.conn, lockFactory: j.lockFactory}
	err := scanBuild(nextBuild, row, j.conn.EncryptionStrategy())
	if err == nil {
		next = nextBuild
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	return next, nil
}

func (j *job) finishedBuild(tx Tx) (Build, error) {
	var finished Build

	row := buildsQuery.
		Where(sq.Eq{"j.id": j.id}).
		Where(sq.Expr("b.id = j.latest_completed_build_id")).
		RunWith(tx).
		QueryRow()

	finishedBuild := &build{conn: j.conn, lockFactory: j.lockFactory}
	err := scanBuild(finishedBuild, row, j.conn.EncryptionStrategy())
	if err == nil {
		finished = finishedBuild
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	return finished, nil
}

func (j *job) getNewRerunBuildName(tx Tx, buildID int) (string, int, error) {
	var rerunNum int
	var buildName string
	err := psql.Select("b.name", "( SELECT COUNT(id) FROM builds WHERE rerun_of = b.id )").
		From("builds b").
		Where(sq.Eq{
			"b.id": buildID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&buildName, &rerunNum)
	if err != nil {
		return "", 0, err
	}

	// increment the rerun number
	rerunNum++

	return buildName + "." + strconv.Itoa(rerunNum), rerunNum, err
}

func (j *job) getNextBuildInputs(tx Tx) ([]BuildInput, error) {
	rows, err := psql.Select("i.input_name, i.first_occurrence, i.resource_id, v.version, i.resolve_error").
		From("next_build_inputs i").
		LeftJoin("resources r ON r.id = i.resource_id").
		LeftJoin("resource_config_versions v ON v.version_md5 = i.version_md5 AND r.resource_config_scope_id = v.resource_config_scope_id").
		Where(sq.Eq{
			"i.job_id": j.id,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	buildInputs := []BuildInput{}
	for rows.Next() {
		var (
			inputName   string
			firstOcc    sql.NullBool
			versionBlob sql.NullString
			resID       sql.NullString
			resolveErr  sql.NullString
		)

		err := rows.Scan(&inputName, &firstOcc, &resID, &versionBlob, &resolveErr)
		if err != nil {
			return nil, err
		}

		var version atc.Version
		if versionBlob.Valid {
			err = json.Unmarshal([]byte(versionBlob.String), &version)
			if err != nil {
				return nil, err
			}
		}

		var firstOccurrence bool
		if firstOcc.Valid {
			firstOccurrence = firstOcc.Bool
		}

		var resourceID int
		if resID.Valid {
			resourceID, err = strconv.Atoi(resID.String)
			if err != nil {
				return nil, err
			}
		}

		var resolveError string
		if resolveErr.Valid {
			resolveError = resolveErr.String
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:            inputName,
			ResourceID:      resourceID,
			Version:         version,
			FirstOccurrence: firstOccurrence,
			ResolveError:    resolveError,
		})
	}

	return buildInputs, err
}

func scanJob(j *job, row scannable) error {
	var (
		configBlob []byte
		nonce      sql.NullString
	)

	err := row.Scan(&j.id, &j.name, &configBlob, &j.paused, &j.firstLoggedBuildID, &j.pipelineID, &j.pipelineName, &j.teamID, &j.teamName, &nonce, pq.Array(&j.tags), &j.hasNewInputs, &j.scheduleRequested)
	if err != nil {
		return err
	}

	es := j.conn.EncryptionStrategy()

	var noncense *string
	if nonce.Valid {
		noncense = &nonce.String
	}

	decryptedConfig, err := es.Decrypt(string(configBlob), noncense)
	if err != nil {
		return err
	}

	var config atc.JobConfig
	err = json.Unmarshal(decryptedConfig, &config)
	if err != nil {
		return err
	}

	j.config = config

	return nil
}

func scanJobs(conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) (Jobs, error) {
	defer Close(rows)

	jobs := Jobs{}

	for rows.Next() {
		job := &job{conn: conn, lockFactory: lockFactory}

		err := scanJob(job, rows)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

func requestSchedule(tx Tx, jobID int) error {
	result, err := psql.Update("jobs").
		Set("schedule_requested", sq.Expr("now()")).
		Where(sq.Eq{
			"id": jobID,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return NonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func requestScheduleOnDownstreamJobs(tx Tx, jobID int) error {
	_, err := psql.Update("jobs").
		Set("schedule_requested", sq.Expr("now()")).
		Where(sq.Expr("id IN (SELECT DISTINCT job_id FROM job_pipes WHERE passed_job_id = $1)", jobID)).
		RunWith(tx).
		Exec()

	return err
}
