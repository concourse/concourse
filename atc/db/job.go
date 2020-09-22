package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/tracing"
	"github.com/lib/pq"
)

type InputConfigs []InputConfig

type InputConfig struct {
	Name            string
	Trigger         bool
	Passed          JobSet
	UseEveryVersion bool
	PinnedVersion   atc.Version
	ResourceID      int
	JobID           int
}

func (cfgs InputConfigs) String() string {
	if !tracing.Configured {
		return ""
	}

	names := make([]string, len(cfgs))
	for i, cfg := range cfgs {
		names[i] = cfg.Name
	}

	return strings.Join(names, ",")
}

type InputVersionEmptyError struct {
	InputName string
}

func (e InputVersionEmptyError) Error() string {
	return fmt.Sprintf("input '%s' has successfully resolved but contains missing version information", e.InputName)
}

//go:generate counterfeiter . Job

type Job interface {
	PipelineRef

	ID() int
	Name() string
	Paused() bool
	FirstLoggedBuildID() int
	TeamID() int
	TeamName() string
	Tags() []string
	Public() bool
	ScheduleRequestedTime() time.Time
	MaxInFlight() int
	DisableManualTrigger() bool

	Config() (atc.JobConfig, error)
	Inputs() ([]atc.JobInput, error)
	Outputs() ([]atc.JobOutput, error)
	AlgorithmInputs() (InputConfigs, error)

	Reload() (bool, error)

	Pause() error
	Unpause() error

	ScheduleBuild(Build) (bool, error)
	CreateBuild() (Build, error)
	RerunBuild(Build) (Build, error)

	RequestSchedule() error
	UpdateLastScheduled(time.Time) error

	Builds(page Page) ([]Build, Pagination, error)
	BuildsWithTime(page Page) ([]Build, Pagination, error)
	Build(name string) (Build, bool, error)
	FinishedAndNextBuild() (Build, Build, error)
	UpdateFirstLoggedBuildID(newFirstLoggedBuildID int) error
	EnsurePendingBuildExists(context.Context) error
	GetPendingBuilds() ([]Build, error)

	GetNextBuildInputs() ([]BuildInput, error)
	GetFullNextBuildInputs() ([]BuildInput, bool, error)
	SaveNextInputMapping(inputMapping InputMapping, inputsDetermined bool) error

	ClearTaskCache(string, string) (int64, error)

	AcquireSchedulingLock(lager.Logger) (lock.Lock, bool, error)

	SetHasNewInputs(bool) error
	HasNewInputs() bool
}

var jobsQuery = psql.Select("j.id", "j.name", "j.config", "j.paused", "j.public", "j.first_logged_build_id", "j.pipeline_id", "p.name", "p.instance_vars", "p.team_id", "t.name", "j.nonce", "j.tags", "j.has_new_inputs", "j.schedule_requested", "j.max_in_flight", "j.disable_manual_trigger").
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
	pipelineRef

	id                    int
	name                  string
	paused                bool
	public                bool
	firstLoggedBuildID    int
	teamID                int
	teamName              string
	tags                  []string
	hasNewInputs          bool
	scheduleRequestedTime time.Time
	maxInFlight           int
	disableManualTrigger  bool

	config    *atc.JobConfig
	rawConfig *string
	nonce     *string
}

func newEmptyJob(conn Conn, lockFactory lock.LockFactory) *job {
	return &job{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
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

func (jobs Jobs) Configs() (atc.JobConfigs, error) {
	var configs atc.JobConfigs

	for _, j := range jobs {
		config, err := j.Config()
		if err != nil {
			return nil, err
		}

		configs = append(configs, config)
	}

	return configs, nil
}

func (j *job) ID() int                          { return j.id }
func (j *job) Name() string                     { return j.name }
func (j *job) Paused() bool                     { return j.paused }
func (j *job) Public() bool                     { return j.public }
func (j *job) FirstLoggedBuildID() int          { return j.firstLoggedBuildID }
func (j *job) TeamID() int                      { return j.teamID }
func (j *job) TeamName() string                 { return j.teamName }
func (j *job) Tags() []string                   { return j.tags }
func (j *job) HasNewInputs() bool               { return j.hasNewInputs }
func (j *job) ScheduleRequestedTime() time.Time { return j.scheduleRequestedTime }
func (j *job) MaxInFlight() int                 { return j.maxInFlight }
func (j *job) DisableManualTrigger() bool       { return j.disableManualTrigger }

func (j *job) Config() (atc.JobConfig, error) {
	if j.config != nil {
		return *j.config, nil
	}

	es := j.conn.EncryptionStrategy()

	if j.rawConfig == nil {
		return atc.JobConfig{}, nil
	}

	decryptedConfig, err := es.Decrypt(*j.rawConfig, j.nonce)
	if err != nil {
		return atc.JobConfig{}, err
	}

	var config atc.JobConfig
	err = json.Unmarshal(decryptedConfig, &config)
	if err != nil {
		return atc.JobConfig{}, err
	}

	j.config = &config
	return config, nil
}

func (j *job) AlgorithmInputs() (InputConfigs, error) {
	rows, err := psql.Select("ji.name", "ji.resource_id", "array_agg(ji.passed_job_id)", "ji.version", "rp.version", "ji.trigger").
		From("job_inputs ji").
		LeftJoin("resource_pins rp ON rp.resource_id = ji.resource_id").
		Where(sq.Eq{
			"ji.job_id": j.id,
		}).
		GroupBy("ji.name, ji.job_id, ji.resource_id, ji.version, rp.version, ji.trigger").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var inputs InputConfigs
	for rows.Next() {
		var passedJobs []sql.NullInt64
		var configVersionString, pinnedVersionString sql.NullString
		var inputName string
		var resourceID int
		var trigger bool

		err = rows.Scan(&inputName, &resourceID, pq.Array(&passedJobs), &configVersionString, &pinnedVersionString, &trigger)
		if err != nil {
			return nil, err
		}

		inputConfig := InputConfig{
			Name:       inputName,
			ResourceID: resourceID,
			JobID:      j.id,
			Trigger:    trigger,
		}

		if pinnedVersionString.Valid {
			err = json.Unmarshal([]byte(pinnedVersionString.String), &inputConfig.PinnedVersion)
			if err != nil {
				return nil, err
			}
		}

		var version *atc.VersionConfig
		if configVersionString.Valid {
			version = &atc.VersionConfig{}
			err = version.UnmarshalJSON([]byte(configVersionString.String))
			if err != nil {
				return nil, err
			}

			inputConfig.UseEveryVersion = version.Every

			if version.Pinned != nil {
				inputConfig.PinnedVersion = version.Pinned
			}
		}

		passed := make(JobSet)
		for _, s := range passedJobs {
			if s.Valid {
				passed[int(s.Int64)] = true
			}
		}

		if len(passed) > 0 {
			inputConfig.Passed = passed
		}

		inputs = append(inputs, inputConfig)
	}

	return inputs, nil
}

func (j *job) Inputs() ([]atc.JobInput, error) {
	rows, err := psql.Select("ji.name", "r.name", "array_agg(p.name ORDER BY p.id)", "ji.trigger", "ji.version").
		From("job_inputs ji").
		Join("resources r ON r.id = ji.resource_id").
		LeftJoin("jobs p ON p.id = ji.passed_job_id").
		Where(sq.Eq{
			"ji.job_id": j.id,
		}).
		GroupBy("ji.name, ji.job_id, r.name, ji.trigger, ji.version").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var inputs []atc.JobInput
	for rows.Next() {
		var passedString []sql.NullString
		var versionString sql.NullString
		var inputName, resourceName string
		var trigger bool

		err = rows.Scan(&inputName, &resourceName, pq.Array(&passedString), &trigger, &versionString)
		if err != nil {
			return nil, err
		}

		var version *atc.VersionConfig
		if versionString.Valid {
			version = &atc.VersionConfig{}
			err = version.UnmarshalJSON([]byte(versionString.String))
			if err != nil {
				return nil, err
			}
		}

		var passed []string
		for _, s := range passedString {
			if s.Valid {
				passed = append(passed, s.String)
			}
		}

		inputs = append(inputs, atc.JobInput{
			Name:     inputName,
			Resource: resourceName,
			Trigger:  trigger,
			Version:  version,
			Passed:   passed,
		})
	}

	sort.Slice(inputs, func(p, q int) bool {
		return inputs[p].Name < inputs[q].Name
	})

	return inputs, nil
}

func (j *job) Outputs() ([]atc.JobOutput, error) {
	rows, err := psql.Select("jo.name", "r.name").
		From("job_outputs jo").
		Join("resources r ON r.id = jo.resource_id").
		Where(sq.Eq{
			"jo.job_id": j.id,
		}).
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	var outputs []atc.JobOutput
	for rows.Next() {
		var output atc.JobOutput
		err = rows.Scan(&output.Name, &output.Resource)
		if err != nil {
			return nil, err
		}

		outputs = append(outputs, output)
	}

	sort.Slice(outputs, func(p, q int) bool {
		return outputs[p].Name < outputs[q].Name
	})

	return outputs, nil
}

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

	build := newEmptyBuild(j.conn, j.lockFactory)

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

	paused, err := j.isPipelineOrJobPaused(tx)
	if err != nil {
		return false, err
	}

	if paused {
		return false, nil
	}

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
			"id": j.id,
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

	defer tx.Rollback()

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

func (j *job) EnsurePendingBuildExists(ctx context.Context) error {
	defer tracing.FromContext(ctx).End()
	spanContextJSON, err := json.Marshal(NewSpanContext(ctx))
	if err != nil {
		return err
	}

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
		INSERT INTO builds (name, job_id, pipeline_id, team_id, status, needs_v6_migration, span_context)
		SELECT $1, $2, $3, $4, 'pending', false, $5
		WHERE NOT EXISTS
			(SELECT id FROM builds WHERE job_id = $2 AND status = 'pending')
		RETURNING id
	`, buildName, j.id, j.pipelineID, j.teamID, string(spanContextJSON))
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

		latestNonRerunID, err := latestCompletedNonRerunBuild(tx, j.id)
		if err != nil {
			return err
		}

		err = updateNextBuildForJob(tx, j.id, latestNonRerunID)
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

	job := newEmptyJob(j.conn, j.lockFactory)
	err := scanJob(job, row)
	if err != nil {
		return nil, err
	}

	rows, err := buildsQuery.
		Where(sq.Eq{
			"b.job_id": j.id,
			"b.status": BuildStatusPending,
		}).
		OrderBy("COALESCE(b.rerun_of, b.id) ASC, b.id ASC").
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		build := newEmptyBuild(j.conn, j.lockFactory)
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

	build := newEmptyBuild(j.conn, j.lockFactory)
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

	latestNonRerunID, err := latestCompletedNonRerunBuild(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.id, latestNonRerunID)
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
	for {
		rerunBuild, err := j.tryRerunBuild(buildToRerun)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
				continue
			}

			return nil, err
		}

		return rerunBuild, nil
	}
}

func (j *job) tryRerunBuild(buildToRerun Build) (Build, error) {
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

	rerunBuild := newEmptyBuild(j.conn, j.lockFactory)
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

	latestNonRerunID, err := latestCompletedNonRerunBuild(tx, j.id)
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.id, latestNonRerunID)
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
	if j.maxInFlight == 0 {
		return false, nil
	}

	serialGroups, err := j.getSerialGroups(tx)
	if err != nil {
		return false, err
	}

	builds, err := j.getRunningBuildsBySerialGroup(tx, serialGroups)
	if err != nil {
		return false, err
	}

	if len(builds) >= j.maxInFlight {
		return true, nil
	}

	nextMostPendingBuild, found, err := j.getNextPendingBuildBySerialGroup(tx, serialGroups)
	if err != nil {
		return false, err
	}

	if !found {
		return true, nil
	}

	if nextMostPendingBuild.ID() != buildID {
		return true, nil
	}

	return false, nil
}

func (j *job) getSerialGroups(tx Tx) ([]string, error) {
	rows, err := psql.Select("serial_group").
		From("jobs_serial_groups").
		Where(sq.Eq{
			"job_id": j.id,
		}).
		RunWith(tx).
		Query()
	if err != nil {
		return nil, err
	}

	var serialGroups []string
	for rows.Next() {
		var serialGroup string
		err = rows.Scan(&serialGroup)
		if err != nil {
			return nil, err
		}

		serialGroups = append(serialGroups, serialGroup)
	}

	return serialGroups, nil
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

func (j *job) UpdateLastScheduled(requestedTime time.Time) error {
	_, err := psql.Update("jobs").
		Set("last_scheduled", requestedTime).
		Where(sq.Eq{
			"id": j.id,
		}).
		RunWith(j.conn).
		Exec()

	return err
}

func (j *job) getRunningBuildsBySerialGroup(tx Tx, serialGroups []string) ([]Build, error) {
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
		build := newEmptyBuild(j.conn, j.lockFactory)
		err = scanBuild(build, rows, j.conn.EncryptionStrategy())
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (j *job) getNextPendingBuildBySerialGroup(tx Tx, serialGroups []string) (Build, bool, error) {
	subQuery, params, err := buildsQuery.Options(`DISTINCT ON (b.id)`).
		Join(`jobs_serial_groups jsg ON j.id = jsg.job_id`).
		Where(sq.Eq{
			"jsg.serial_group":    serialGroups,
			"b.status":            BuildStatusPending,
			"j.paused":            false,
			"j.inputs_determined": true,
			"j.pipeline_id":       j.pipelineID}).
		ToSql()
	if err != nil {
		return nil, false, err
	}

	row := tx.QueryRow(`
			SELECT * FROM (`+subQuery+`) j
			ORDER BY COALESCE(rerun_of, id) ASC, id ASC
			LIMIT 1`, params...)

	build := newEmptyBuild(j.conn, j.lockFactory)
	err = scanBuild(build, row, j.conn.EncryptionStrategy())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return build, true, nil
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

	if !pause {
		err = j.RequestSchedule()
		if err != nil {
			return err
		}
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

	nextBuild := newEmptyBuild(j.conn, j.lockFactory)
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

	finishedBuild := newEmptyBuild(j.conn, j.lockFactory)
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
	rows, err := psql.Select("i.input_name, i.first_occurrence, i.resource_id, v.version, i.resolve_error, v.span_context").
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
			inputName       string
			firstOcc        sql.NullBool
			versionBlob     sql.NullString
			resID           sql.NullString
			resolveErr      sql.NullString
			spanContextJSON sql.NullString
		)

		err := rows.Scan(&inputName, &firstOcc, &resID, &versionBlob, &resolveErr, &spanContextJSON)
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

		var spanContext SpanContext
		if spanContextJSON.Valid {
			err = json.Unmarshal([]byte(spanContextJSON.String), &spanContext)
			if err != nil {
				return nil, err
			}
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:            inputName,
			ResourceID:      resourceID,
			Version:         version,
			FirstOccurrence: firstOccurrence,
			ResolveError:    resolveError,
			Context:         spanContext,
		})
	}

	return buildInputs, err
}

func (j *job) isPipelineOrJobPaused(tx Tx) (bool, error) {
	if j.paused {
		return true, nil
	}

	var paused bool
	err := psql.Select("paused").
		From("pipelines").
		Where(sq.Eq{"id": j.pipelineID}).
		RunWith(tx).
		QueryRow().
		Scan(&paused)
	if err != nil {
		return false, err
	}

	return paused, nil
}

func scanJob(j *job, row scannable) error {
	var (
		config               sql.NullString
		nonce                sql.NullString
		pipelineInstanceVars sql.NullString
	)

	err := row.Scan(&j.id, &j.name, &config, &j.paused, &j.public, &j.firstLoggedBuildID, &j.pipelineID, &j.pipelineName, &pipelineInstanceVars, &j.teamID, &j.teamName, &nonce, pq.Array(&j.tags), &j.hasNewInputs, &j.scheduleRequestedTime, &j.maxInFlight, &j.disableManualTrigger)
	if err != nil {
		return err
	}

	if nonce.Valid {
		j.nonce = &nonce.String
	}

	if config.Valid {
		j.rawConfig = &config.String
	}

	if pipelineInstanceVars.Valid {
		err = json.Unmarshal([]byte(pipelineInstanceVars.String), &j.pipelineInstanceVars)
		if err != nil {
			return err
		}
	}

	return nil
}

func scanJobs(conn Conn, lockFactory lock.LockFactory, rows *sql.Rows) (Jobs, error) {
	defer Close(rows)

	jobs := Jobs{}

	for rows.Next() {
		job := newEmptyJob(conn, lockFactory)
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

// The SELECT query orders the jobs for updating to prevent deadlocking.
// Updating multiple rows using a SELECT subquery does not preserve the same
// order for the updates, which can lead to deadlocking.
func requestScheduleOnDownstreamJobs(tx Tx, jobID int) error {
	rows, err := psql.Select("DISTINCT job_id").
		From("job_inputs").
		Where(sq.Eq{
			"passed_job_id": jobID,
		}).
		OrderBy("job_id DESC").
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}

	var jobIDs []int
	for rows.Next() {
		var id int
		err = rows.Scan(&id)
		if err != nil {
			return err
		}

		jobIDs = append(jobIDs, id)
	}

	for _, jID := range jobIDs {
		_, err := psql.Update("jobs").
			Set("schedule_requested", sq.Expr("now()")).
			Where(sq.Eq{
				"id": jID,
			}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
