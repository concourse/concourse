package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/lib/pq"
)

const schema = "exec.v2"

type BuildInput struct {
	Name       string
	Version    atc.Version
	ResourceID int

	FirstOccurrence bool
}

type BuildOutput struct {
	Name    string
	Version atc.Version
}

type BuildStatus string

const (
	BuildStatusPending   BuildStatus = "pending"
	BuildStatusStarted   BuildStatus = "started"
	BuildStatusAborted   BuildStatus = "aborted"
	BuildStatusSucceeded BuildStatus = "succeeded"
	BuildStatusFailed    BuildStatus = "failed"
	BuildStatusErrored   BuildStatus = "errored"
)

var buildsQuery = psql.Select("b.id, b.name, b.job_id, b.team_id, b.status, b.manually_triggered, b.scheduled, b.schema, b.private_plan, b.public_plan, b.create_time, b.start_time, b.end_time, b.reap_time, j.name, b.pipeline_id, p.name, t.name, b.nonce, b.drained, b.aborted, b.completed").
	From("builds b").
	JoinClause("LEFT OUTER JOIN jobs j ON b.job_id = j.id").
	JoinClause("LEFT OUTER JOIN pipelines p ON b.pipeline_id = p.id").
	JoinClause("LEFT OUTER JOIN teams t ON b.team_id = t.id")

var minMaxIdQuery = psql.Select("COALESCE(MAX(b.id), 0)", "COALESCE(MIN(b.id), 0)").
	From("builds as b")

//go:generate counterfeiter . Build

type Build interface {
	ID() int
	Name() string
	JobID() int
	JobName() string
	PipelineID() int
	PipelineName() string
	TeamID() int
	TeamName() string
	Schema() string
	PrivatePlan() atc.Plan
	PublicPlan() *json.RawMessage
	HasPlan() bool
	Status() BuildStatus
	StartTime() time.Time
	IsNewerThanLastCheckOf(input Resource) bool
	EndTime() time.Time
	ReapTime() time.Time
	IsManuallyTriggered() bool
	IsScheduled() bool
	IsRunning() bool
	IsCompleted() bool

	Reload() (bool, error)

	AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error)

	Interceptible() (bool, error)
	Preparation() (BuildPreparation, bool, error)

	Start(atc.Plan) (bool, error)
	Finish(BuildStatus) error

	SetInterceptible(bool) error

	Events(uint) (EventSource, error)
	SaveEvent(event atc.Event) error

	Artifacts() ([]WorkerArtifact, error)
	Artifact(artifactID int) (WorkerArtifact, error)

	SaveOutput(string, atc.Source, atc.VersionedResourceTypes, atc.Version, ResourceConfigMetadataFields, string, string) error
	UseInputs(inputs []BuildInput) error

	Resources() ([]BuildInput, []BuildOutput, error)
	SaveImageResourceVersion(UsedResourceCache) error

	Pipeline() (Pipeline, bool, error)

	Delete() (bool, error)
	MarkAsAborted() error
	IsAborted() bool
	AbortNotifier() (Notifier, error)
	Schedule() (bool, error)

	IsDrained() bool
	SetDrained(bool) error
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

	schema      string
	privatePlan atc.Plan
	publicPlan  *json.RawMessage

	createTime time.Time
	startTime  time.Time
	endTime    time.Time
	reapTime   time.Time

	conn        Conn
	lockFactory lock.LockFactory
	drained     bool
	aborted     bool
	completed   bool
}

var ErrBuildDisappeared = errors.New("build disappeared from db")
var ErrBuildHasNoPipeline = errors.New("build has no pipeline")
var ErrBuildArtifactNotFound = errors.New("build artifact not found")

type ResourceNotFoundInPipeline struct {
	Resource string
	Pipeline string
}

func (r ResourceNotFoundInPipeline) Error() string {
	return fmt.Sprintf("resource %s not found in pipeline %s", r.Resource, r.Pipeline)
}

func (b *build) ID() int                      { return b.id }
func (b *build) Name() string                 { return b.name }
func (b *build) JobID() int                   { return b.jobID }
func (b *build) JobName() string              { return b.jobName }
func (b *build) PipelineID() int              { return b.pipelineID }
func (b *build) PipelineName() string         { return b.pipelineName }
func (b *build) TeamID() int                  { return b.teamID }
func (b *build) TeamName() string             { return b.teamName }
func (b *build) IsManuallyTriggered() bool    { return b.isManuallyTriggered }
func (b *build) Schema() string               { return b.schema }
func (b *build) PrivatePlan() atc.Plan        { return b.privatePlan }
func (b *build) PublicPlan() *json.RawMessage { return b.publicPlan }
func (b *build) HasPlan() bool                { return string(*b.publicPlan) != "{}" }
func (b *build) IsNewerThanLastCheckOf(input Resource) bool {
	return b.createTime.After(input.LastCheckEndTime())
}
func (b *build) StartTime() time.Time { return b.startTime }
func (b *build) EndTime() time.Time   { return b.endTime }
func (b *build) ReapTime() time.Time  { return b.reapTime }
func (b *build) Status() BuildStatus  { return b.status }
func (b *build) IsScheduled() bool    { return b.scheduled }
func (b *build) IsDrained() bool      { return b.drained }
func (b *build) IsRunning() bool      { return !b.completed }
func (b *build) IsAborted() bool      { return b.aborted }
func (b *build) IsCompleted() bool    { return b.completed }

func (b *build) Reload() (bool, error) {
	row := buildsQuery.Where(sq.Eq{"b.id": b.id}).
		RunWith(b.conn).
		QueryRow()

	err := scanBuild(b, row, b.conn.EncryptionStrategy())
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

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

func (b *build) Start(plan atc.Plan) (bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return false, err
	}

	defer Rollback(tx)

	metadata, err := json.Marshal(plan)
	if err != nil {
		return false, err
	}

	encryptedPlan, nonce, err := b.conn.EncryptionStrategy().Encrypt([]byte(metadata))
	if err != nil {
		return false, err
	}

	var startTime time.Time

	err = psql.Update("builds").
		Set("status", BuildStatusStarted).
		Set("start_time", sq.Expr("now()")).
		Set("schema", schema).
		Set("private_plan", encryptedPlan).
		Set("public_plan", plan.Public()).
		Set("nonce", nonce).
		Where(sq.Eq{
			"id":      b.id,
			"status":  "pending",
			"aborted": false,
		}).
		Suffix("RETURNING start_time").
		RunWith(tx).
		QueryRow().
		Scan(&startTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	err = b.saveEvent(tx, event.Status{
		Status: atc.StatusStarted,
		Time:   startTime.Unix(),
	})
	if err != nil {
		return false, err
	}

	if b.jobID != 0 {
		err = updateNextBuildForJob(tx, b.jobID)
		if err != nil {
			return false, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return false, err
	}

	return true, nil
}

func (b *build) Finish(status BuildStatus) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	var endTime time.Time

	err = psql.Update("builds").
		Set("status", status).
		Set("end_time", sq.Expr("now()")).
		Set("completed", true).
		Set("private_plan", nil).
		Set("nonce", nil).
		Where(sq.Eq{"id": b.id}).
		Suffix("RETURNING end_time").
		RunWith(tx).
		QueryRow().
		Scan(&endTime)
	if err != nil {
		return err
	}

	err = b.saveEvent(tx, event.Status{
		Status: atc.BuildStatus(status),
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

	if b.jobID != 0 && status == BuildStatusSucceeded {
		_, err = psql.Delete("build_image_resource_caches birc USING builds b").
			Where(sq.Expr("birc.build_id = b.id")).
			Where(sq.Lt{"build_id": b.id}).
			Where(sq.Eq{"b.job_id": b.jobID}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	if b.jobID != 0 {
		err = bumpCacheIndex(tx, b.pipelineID)
		if err != nil {
			return err
		}

		err = updateTransitionBuildForJob(tx, b.jobID, b.id, status)
		if err != nil {
			return err
		}

		err = updateLatestCompletedBuildForJob(tx, b.jobID)
		if err != nil {
			return err
		}

		err = updateNextBuildForJob(tx, b.jobID)
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return err
	}

	return nil
}

func (b *build) SetDrained(drained bool) error {
	_, err := psql.Update("builds").
		Set("drained", drained).
		Where(sq.Eq{"id": b.id}).
		RunWith(b.conn).
		Exec()

	if err == nil {
		b.drained = drained
	}
	return err
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

// MarkAsAborted will send the abort notification to all build abort
// channel listeners. It will set the status to aborted that will make
// AbortNotifier send notification in case if tracking ATC misses the first
// notification on abort channel.
// Setting status as aborted will also make Start() return false in case where
// build was aborted before it was started.
func (b *build) MarkAsAborted() error {
	_, err := psql.Update("builds").
		Set("aborted", true).
		Where(sq.Eq{"id": b.id}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return err
	}

	return b.conn.Bus().Notify(buildAbortChannel(b.id))
}

// AbortNotifier returns a Notifier that can be watched for when the build
// is marked as aborted. Once the build is marked as aborted it will send a
// notification to finish the build to ATC that is tracking this build.
func (b *build) AbortNotifier() (Notifier, error) {
	return newConditionNotifier(b.conn.Bus(), buildAbortChannel(b.id), func() (bool, error) {
		var aborted bool
		err := psql.Select("aborted = true").
			From("builds").
			Where(sq.Eq{"id": b.id}).
			RunWith(b.conn).
			QueryRow().
			Scan(&aborted)

		return aborted, err
	})
}

func (b *build) Schedule() (bool, error) {
	result, err := psql.Update("builds").
		Set("scheduled", true).
		Where(sq.Eq{"id": b.id}).
		RunWith(b.conn).
		Exec()
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (b *build) Pipeline() (Pipeline, bool, error) {
	if b.pipelineID == 0 {
		return nil, false, nil
	}

	row := pipelinesQuery.
		Where(sq.Eq{"p.id": b.pipelineID}).
		RunWith(b.conn).
		QueryRow()

	pipeline := newPipeline(b.conn, b.lockFactory)
	err := scanPipeline(pipeline, row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, err
	}

	return pipeline, true, nil
}

func (b *build) SaveImageResourceVersion(rc UsedResourceCache) error {
	_, err := psql.Insert("build_image_resource_caches").
		Columns("resource_cache_id", "build_id").
		Values(rc.ID(), b.id).
		RunWith(b.conn).
		Exec()
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
			return nil
		}

		return err
	}

	return nil
}

func (b *build) AcquireTrackingLock(logger lager.Logger, interval time.Duration) (lock.Lock, bool, error) {
	lock, acquired, err := b.lockFactory.Acquire(
		logger.Session("lock", lager.Data{
			"build_id": b.id,
		}),
		lock.NewBuildTrackingLockID(b.id),
	)
	if err != nil {
		return nil, false, err
	}

	if !acquired {
		return nil, false, nil
	}

	return lock, true, nil
}

func (b *build) Preparation() (BuildPreparation, bool, error) {
	if b.jobID == 0 || b.status != BuildStatusPending {
		return BuildPreparation{
			BuildID:             b.id,
			PausedPipeline:      BuildPreparationStatusNotBlocking,
			PausedJob:           BuildPreparationStatusNotBlocking,
			MaxRunningBuilds:    BuildPreparationStatusNotBlocking,
			Inputs:              map[string]BuildPreparationStatus{},
			InputsSatisfied:     BuildPreparationStatusNotBlocking,
			MissingInputReasons: MissingInputReasons{},
		}, true, nil
	}

	var (
		pausedPipeline     bool
		pausedJob          bool
		maxInFlightReached bool
		pipelineID         int
		jobName            string
	)
	err := psql.Select("p.paused, j.paused, j.max_in_flight_reached, j.pipeline_id, j.name").
		From("builds b").
		Join("jobs j ON b.job_id = j.id").
		Join("pipelines p ON j.pipeline_id = p.id").
		Where(sq.Eq{"b.id": b.id}).
		RunWith(b.conn).
		QueryRow().
		Scan(&pausedPipeline, &pausedJob, &maxInFlightReached, &pipelineID, &jobName)
	if err != nil {
		if err == sql.ErrNoRows {
			return BuildPreparation{}, false, nil
		}
		return BuildPreparation{}, false, err
	}

	pausedPipelineStatus := BuildPreparationStatusNotBlocking
	if pausedPipeline {
		pausedPipelineStatus = BuildPreparationStatusBlocking
	}

	pausedJobStatus := BuildPreparationStatusNotBlocking
	if pausedJob {
		pausedJobStatus = BuildPreparationStatusBlocking
	}

	maxInFlightReachedStatus := BuildPreparationStatusNotBlocking
	if maxInFlightReached {
		maxInFlightReachedStatus = BuildPreparationStatusBlocking
	}

	tf := NewTeamFactory(b.conn, b.lockFactory)
	t, found, err := tf.FindTeam(b.teamName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	pipeline, found, err := t.Pipeline(b.pipelineName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	job, found, err := pipeline.Job(jobName)
	if err != nil {
		return BuildPreparation{}, false, err
	}

	if !found {
		return BuildPreparation{}, false, nil
	}

	configInputs := job.Config().Inputs()

	nextBuildInputs, found, err := job.GetNextBuildInputs()
	if err != nil {
		return BuildPreparation{}, false, err
	}

	inputsSatisfiedStatus := BuildPreparationStatusBlocking
	inputs := map[string]BuildPreparationStatus{}
	missingInputReasons := MissingInputReasons{}

	if found {

		inputsSatisfiedStatus = BuildPreparationStatusNotBlocking

		if b.IsManuallyTriggered() {
			for _, buildInput := range nextBuildInputs {
				resource, _, err := pipeline.ResourceByID(buildInput.ResourceID)
				if err != nil {
					return BuildPreparation{}, false, err
				}

				// input is blocking if its last check time is before build create time
				if b.IsNewerThanLastCheckOf(resource) {
					inputs[buildInput.Name] = BuildPreparationStatusBlocking
					missingInputReasons.RegisterNoResourceCheckFinished(buildInput.Name)
					inputsSatisfiedStatus = BuildPreparationStatusBlocking
				} else {
					inputs[buildInput.Name] = BuildPreparationStatusNotBlocking
				}
			}
		} else {
			for _, buildInput := range nextBuildInputs {
				inputs[buildInput.Name] = BuildPreparationStatusNotBlocking
			}
		}
	} else {
		buildInputs, err := job.GetIndependentBuildInputs()
		if err != nil {
			return BuildPreparation{}, false, err
		}

		for _, configInput := range configInputs {
			found := false
			for _, buildInput := range buildInputs {
				if buildInput.Name == configInput.Name {
					found = true
					break
				}
			}
			if found {
				inputs[configInput.Name] = BuildPreparationStatusNotBlocking
			} else {
				inputs[configInput.Name] = BuildPreparationStatusBlocking
				if len(configInput.Passed) > 0 {
					if configInput.Version != nil && configInput.Version.Pinned != nil {
						versionJSON, err := json.Marshal(configInput.Version.Pinned)
						if err != nil {
							return BuildPreparation{}, false, err
						}

						resource, found, err := pipeline.Resource(configInput.Resource)
						if err != nil {
							return BuildPreparation{}, false, err
						}

						if found {
							_, found, err = resource.ResourceConfigVersionID(configInput.Version.Pinned)
							if err != nil {
								return BuildPreparation{}, false, err
							}

							if found {
								missingInputReasons.RegisterPassedConstraint(configInput.Name)
							} else {
								missingInputReasons.RegisterPinnedVersionUnavailable(configInput.Name, string(versionJSON))
							}
						} else {
							missingInputReasons.RegisterPinnedVersionUnavailable(configInput.Name, string(versionJSON))
						}
					} else {
						missingInputReasons.RegisterPassedConstraint(configInput.Name)
					}
				} else {
					if configInput.Version != nil && configInput.Version.Pinned != nil {
						versionJSON, err := json.Marshal(configInput.Version.Pinned)
						if err != nil {
							return BuildPreparation{}, false, err
						}

						missingInputReasons.RegisterPinnedVersionUnavailable(configInput.Name, string(versionJSON))
					} else {
						missingInputReasons.RegisterNoVersions(configInput.Name)
					}
				}
			}
		}
	}

	buildPreparation := BuildPreparation{
		BuildID:             b.id,
		PausedPipeline:      pausedPipelineStatus,
		PausedJob:           pausedJobStatus,
		MaxRunningBuilds:    maxInFlightReachedStatus,
		Inputs:              inputs,
		InputsSatisfied:     inputsSatisfiedStatus,
		MissingInputReasons: missingInputReasons,
	}

	return buildPreparation, true, nil
}

func (b *build) Events(from uint) (EventSource, error) {
	notifier, err := newConditionNotifier(b.conn.Bus(), buildEventsChannel(b.id), func() (bool, error) {
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	table := fmt.Sprintf("team_build_events_%d", b.teamID)
	if b.pipelineID != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", b.pipelineID)
	}

	return newBuildEventSource(
		b.id,
		table,
		b.conn,
		notifier,
		from,
	), nil
}

func (b *build) SaveEvent(event atc.Event) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	err = b.saveEvent(tx, event)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return b.conn.Bus().Notify(buildEventsChannel(b.id))
}

func (b *build) Artifact(artifactID int) (WorkerArtifact, error) {

	artifact := artifact{
		conn: b.conn,
	}

	err := psql.Select("id", "name", "created_at").
		From("worker_artifacts").
		Where(sq.Eq{
			"id": artifactID,
		}).
		RunWith(b.conn).
		Scan(&artifact.id, &artifact.name, &artifact.createdAt)

	return &artifact, err
}

func (b *build) Artifacts() ([]WorkerArtifact, error) {
	artifacts := []WorkerArtifact{}

	rows, err := psql.Select("id", "name", "created_at").
		From("worker_artifacts").
		Where(sq.Eq{
			"build_id": b.id,
		}).
		RunWith(b.conn).
		Query()
	if err != nil {
		return nil, err
	}

	defer Close(rows)

	for rows.Next() {
		wa := artifact{
			conn:    b.conn,
			buildID: b.id,
		}

		err = rows.Scan(&wa.id, &wa.name, &wa.createdAt)
		if err != nil {
			return nil, err
		}

		artifacts = append(artifacts, &wa)
	}

	return artifacts, nil
}

func (b *build) SaveOutput(
	resourceType string,
	source atc.Source,
	resourceTypes atc.VersionedResourceTypes,
	version atc.Version,
	metadata ResourceConfigMetadataFields,
	outputName string,
	resourceName string,
) error {
	// We should never save outputs for builds without a Pipeline ID because
	// One-off Builds will never have Put steps. This shouldn't happen, but
	// its best to return an error just in case
	if b.pipelineID == 0 {
		return ErrBuildHasNoPipeline
	}

	pipeline, found, err := b.Pipeline()
	if err != nil {
		return err
	}

	if !found {
		return ErrBuildHasNoPipeline
	}

	resource, found, err := pipeline.Resource(resourceName)
	if err != nil {
		return err
	}

	if !found {
		return ResourceNotFoundInPipeline{resourceName, b.pipelineName}
	}

	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	resourceConfigDescriptor, err := constructResourceConfigDescriptor(resourceType, source, resourceTypes)
	if err != nil {
		return err
	}

	resourceConfig, err := resourceConfigDescriptor.findOrCreate(tx, b.lockFactory, b.conn)
	if err != nil {
		return err
	}

	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, b.conn, b.lockFactory, resourceConfig, resource, resourceTypes)
	if err != nil {
		return err
	}

	newVersion, err := saveResourceVersion(tx, resourceConfigScope, version, metadata)
	if err != nil {
		return err
	}

	versionBytes, err := json.Marshal(version)
	if err != nil {
		return err
	}

	versionJSON := string(versionBytes)

	if newVersion {
		err = incrementCheckOrder(tx, resourceConfigScope, versionJSON)
		if err != nil {
			return err
		}
	}

	_, err = psql.Insert("build_resource_config_version_outputs").
		Columns("resource_id", "build_id", "version_md5", "name").
		Values(resource.ID(), strconv.Itoa(b.id), sq.Expr("md5(?)", versionJSON), outputName).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(tx).
		Exec()

	if err != nil {
		return err
	}

	err = bumpCacheIndex(tx, b.pipelineID)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	err = bumpCacheIndexForPipelinesUsingResourceConfigScope(b.conn, resourceConfigScope.ID())
	if err != nil {
		return err
	}

	return nil
}

func (b *build) UseInputs(inputs []BuildInput) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Delete("build_resource_config_version_inputs").
		Where(sq.Eq{"build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return err
	}

	for _, input := range inputs {
		err = b.saveInputTx(tx, b.id, input)
		if err != nil {
			return err
		}
	}

	if b.pipelineID != 0 {
		err = bumpCacheIndex(tx, b.pipelineID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (b *build) Resources() ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	firstOccurrence := `
		NOT EXISTS (
			SELECT 1
			FROM build_resource_config_version_inputs i, builds b
			WHERE versions.version_md5 = i.version_md5
			AND resources.resource_config_scope_id = versions.resource_config_scope_id
			AND resources.id = i.resource_id
			AND b.job_id = builds.job_id
			AND i.build_id = b.id
			AND i.build_id < builds.id
		)`

	rows, err := psql.Select("inputs.name", "resources.id", "versions.version", firstOccurrence).
		From("resource_config_versions versions, build_resource_config_version_inputs inputs, builds, resources").
		Where(sq.Eq{"builds.id": b.id}).
		Where(sq.NotEq{"versions.check_order": 0}).
		Where(sq.Expr("inputs.build_id = builds.id")).
		Where(sq.Expr("inputs.version_md5 = versions.version_md5")).
		Where(sq.Expr("resources.resource_config_scope_id = versions.resource_config_scope_id")).
		Where(sq.Expr("resources.id = inputs.resource_id")).
		Where(sq.Expr(`NOT EXISTS (
			SELECT 1
			FROM build_resource_config_version_outputs outputs
			WHERE outputs.version_md5 = versions.version_md5
			AND versions.resource_config_scope_id = resources.resource_config_scope_id
			AND outputs.resource_id = resources.id
			AND outputs.build_id = inputs.build_id
		)`)).
		RunWith(b.conn).
		Query()
	if err != nil {
		return nil, nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var (
			inputName       string
			firstOccurrence bool
			versionBlob     string
			version         atc.Version
			resourceID      int
		)

		err = rows.Scan(&inputName, &resourceID, &versionBlob, &firstOccurrence)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, nil, err
		}

		inputs = append(inputs, BuildInput{
			Name:            inputName,
			Version:         version,
			ResourceID:      resourceID,
			FirstOccurrence: firstOccurrence,
		})
	}

	rows, err = psql.Select("outputs.name", "versions.version").
		From("resource_config_versions versions, build_resource_config_version_outputs outputs, builds, resources").
		Where(sq.Eq{"builds.id": b.id}).
		Where(sq.NotEq{"versions.check_order": 0}).
		Where(sq.Expr("outputs.build_id = builds.id")).
		Where(sq.Expr("outputs.version_md5 = versions.version_md5")).
		Where(sq.Expr("outputs.resource_id = resources.id")).
		Where(sq.Expr("resources.resource_config_scope_id = versions.resource_config_scope_id")).
		RunWith(b.conn).
		Query()

	if err != nil {
		return nil, nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var (
			outputName  string
			versionBlob string
			version     atc.Version
		)

		err := rows.Scan(&outputName, &versionBlob)
		if err != nil {
			return nil, nil, err
		}

		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, nil, err
		}

		outputs = append(outputs, BuildOutput{
			Name:    outputName,
			Version: version,
		})
	}

	return inputs, outputs, nil
}

func (p *build) saveInputTx(tx Tx, buildID int, input BuildInput) error {
	versionJSON, err := json.Marshal(input.Version)
	if err != nil {
		return err
	}

	_, err = psql.Insert("build_resource_config_version_inputs").
		Columns("build_id", "resource_id", "version_md5", "name").
		Values(buildID, input.ResourceID, sq.Expr("md5(?)", versionJSON), input.Name).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(tx).
		Exec()

	return err
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

func scanBuild(b *build, row scannable, encryptionStrategy encryption.Strategy) error {
	var (
		jobID, pipelineID                                      sql.NullInt64
		schema, privatePlan, jobName, pipelineName, publicPlan sql.NullString
		createTime, startTime, endTime, reapTime               pq.NullTime
		nonce                                                  sql.NullString
		drained, aborted, completed                            bool
		status                                                 string
	)

	err := row.Scan(&b.id, &b.name, &jobID, &b.teamID, &status, &b.isManuallyTriggered, &b.scheduled, &schema, &privatePlan, &publicPlan, &createTime, &startTime, &endTime, &reapTime, &jobName, &pipelineID, &pipelineName, &b.teamName, &nonce, &drained, &aborted, &completed)
	if err != nil {
		return err
	}

	b.status = BuildStatus(status)
	b.jobName = jobName.String
	b.jobID = int(jobID.Int64)
	b.pipelineName = pipelineName.String
	b.pipelineID = int(pipelineID.Int64)
	b.schema = schema.String
	b.createTime = createTime.Time
	b.startTime = startTime.Time
	b.endTime = endTime.Time
	b.reapTime = reapTime.Time
	b.drained = drained
	b.aborted = aborted
	b.completed = completed

	var (
		noncense      *string
		decryptedPlan []byte
	)

	if nonce.Valid {
		noncense = &nonce.String
		decryptedPlan, err = encryptionStrategy.Decrypt(string(privatePlan.String), noncense)
		if err != nil {
			return err
		}
	} else {
		decryptedPlan = []byte(privatePlan.String)
	}

	if len(decryptedPlan) > 0 {
		err = json.Unmarshal(decryptedPlan, &b.privatePlan)
		if err != nil {
			return err
		}
	}

	if publicPlan.Valid {
		err = json.Unmarshal([]byte(publicPlan.String), &b.publicPlan)
		if err != nil {
			return err
		}
	}

	return nil
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
	_, err = psql.Insert(table).
		Columns("event_id", "build_id", "type", "version", "payload").
		Values(sq.Expr("nextval('"+buildEventSeq(b.id)+"')"), b.id, string(event.EventType()), string(event.Version()), payload).
		RunWith(tx).
		Exec()
	return err
}

func createBuild(tx Tx, build *build, vals map[string]interface{}) error {
	var buildID int
	err := psql.Insert("builds").
		SetMap(vals).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		return err
	}

	err = scanBuild(build, buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(tx).
		QueryRow(),
		build.conn.EncryptionStrategy(),
	)
	if err != nil {
		return err
	}

	return createBuildEventSeq(tx, buildID)
}

func buildStartedChannel() string {
	return fmt.Sprintf("build_started")
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}

func updateNextBuildForJob(tx Tx, jobID int) error {
	_, err := tx.Exec(`
		UPDATE jobs AS j
		SET next_build_id = (
			SELECT min(b.id)
			FROM builds b
			WHERE b.job_id = $1
			AND b.status IN ('pending', 'started')
		)
		WHERE j.id = $1
	`, jobID)
	if err != nil {
		return err
	}
	return nil
}

func updateLatestCompletedBuildForJob(tx Tx, jobID int) error {
	_, err := tx.Exec(`
		UPDATE jobs AS j
		SET latest_completed_build_id = (
			SELECT max(b.id)
			FROM builds b
			WHERE b.job_id = $1
			AND b.status NOT IN ('pending', 'started')
		)
		WHERE j.id = $1
	`, jobID)
	if err != nil {
		return err
	}

	return nil
}

func updateTransitionBuildForJob(tx Tx, jobID int, buildID int, buildStatus BuildStatus) error {
	var shouldUpdateTransition bool

	var latestID int
	var latestStatus BuildStatus
	err := psql.Select("b.id", "b.status").
		From("builds b").
		JoinClause("INNER JOIN jobs j ON j.latest_completed_build_id = b.id").
		Where(sq.Eq{"j.id": jobID}).
		RunWith(tx).
		QueryRow().
		Scan(&latestID, &latestStatus)
	if err != nil {
		if err == sql.ErrNoRows {
			// this is the first completed build; initiate transition
			shouldUpdateTransition = true
		} else {
			return err
		}
	}

	if buildID < latestID {
		// latest completed build is actually after this one, so this build
		// has no influence on the job's overall state
		//
		// this can happen when multiple builds are running at a time and the
		// later-queued ones finish earlier
		return nil
	}

	if latestStatus != buildStatus {
		// status has changed; transitioned!
		shouldUpdateTransition = true
	}

	if shouldUpdateTransition {
		_, err := psql.Update("jobs").
			Set("transition_build_id", buildID).
			Where(sq.Eq{"id": jobID}).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}
	}

	return nil
}
