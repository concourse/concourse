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
	"github.com/lib/pq"
	"go.opentelemetry.io/otel/api/propagators"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
)

const schema = "exec.v2"

var ErrAdoptRerunBuildHasNoInputs = errors.New("inputs not ready for build to rerun")
var ErrSetByNewerBuild = errors.New("pipeline set by a newer build")

type BuildInput struct {
	Name       string
	Version    atc.Version
	ResourceID int

	FirstOccurrence bool
	ResolveError    string

	Context SpanContext
}

func (bi BuildInput) SpanContext() propagators.Supplier {
	return bi.Context
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

var buildsQuery = psql.Select(`
		b.id,
		b.name,
		b.job_id,
		b.team_id,
		b.status,
		b.manually_triggered,
		b.scheduled,
		b.schema,
		b.private_plan,
		b.public_plan,
		b.create_time,
		b.start_time,
		b.end_time,
		b.reap_time,
		j.name,
		b.pipeline_id,
		p.name,
		t.name,
		b.nonce,
		b.drained,
		b.aborted,
		b.completed,
		b.inputs_ready,
		b.rerun_of,
		r.name,
		b.rerun_number,
		b.span_context
	`).
	From("builds b").
	JoinClause("LEFT OUTER JOIN jobs j ON b.job_id = j.id").
	JoinClause("LEFT OUTER JOIN pipelines p ON b.pipeline_id = p.id").
	JoinClause("LEFT OUTER JOIN teams t ON b.team_id = t.id").
	JoinClause("LEFT OUTER JOIN builds r ON r.id = b.rerun_of")

var minMaxIdQuery = psql.Select("COALESCE(MAX(b.id), 0)", "COALESCE(MIN(b.id), 0)").
	From("builds as b")

var latestCompletedBuildQuery = psql.Select("max(id)").
	From("builds").
	Where(sq.Expr(`status NOT IN ('pending', 'started')`))

//go:generate counterfeiter . Build

type Build interface {
	PipelineRef

	ID() int
	Name() string
	JobID() int
	JobName() string
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
	InputsReady() bool
	RerunOf() int
	RerunOfName() string
	RerunNumber() int

	Reload() (bool, error)

	ResourcesChecked() (bool, error)

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
	AdoptInputsAndPipes() ([]BuildInput, bool, error)
	AdoptRerunInputsAndPipes() ([]BuildInput, bool, error)

	Resources() ([]BuildInput, []BuildOutput, error)
	SaveImageResourceVersion(UsedResourceCache) error

	Delete() (bool, error)
	MarkAsAborted() error
	IsAborted() bool
	AbortNotifier() (Notifier, error)

	IsDrained() bool
	SetDrained(bool) error

	SpanContext() propagators.Supplier

	SavePipeline(
		pipelineName string,
		teamId int,
		config atc.Config,
		from ConfigVersion,
		initiallyPaused bool,
	) (Pipeline, bool, error)
}

type build struct {
	pipelineRef

	id          int
	name        string
	status      BuildStatus
	scheduled   bool
	inputsReady bool

	teamID   int
	teamName string

	jobID   int
	jobName string

	isManuallyTriggered bool

	rerunOf     int
	rerunOfName string
	rerunNumber int

	schema      string
	privatePlan atc.Plan
	publicPlan  *json.RawMessage

	createTime time.Time
	startTime  time.Time
	endTime    time.Time
	reapTime   time.Time

	drained   bool
	aborted   bool
	completed bool

	spanContext SpanContext
}

func newEmptyBuild(conn Conn, lockFactory lock.LockFactory) *build {
	return &build{pipelineRef: pipelineRef{conn: conn, lockFactory: lockFactory}}
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
func (b *build) InputsReady() bool    { return b.inputsReady }
func (b *build) RerunOf() int         { return b.rerunOf }
func (b *build) RerunOfName() string  { return b.rerunOfName }
func (b *build) RerunNumber() int     { return b.rerunNumber }

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

func (b *build) ResourcesChecked() (bool, error) {
	var notChecked bool
	err := b.conn.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM resources r
			JOIN job_inputs ji ON ji.resource_id = r.id
			JOIN resource_config_scopes rs ON r.resource_config_scope_id = rs.id
			WHERE ji.job_id = $1
			AND rs.last_check_end_time < $2
			AND NOT EXISTS (
				SELECT
				FROM resource_pins
				WHERE resource_id = r.id
			)
		)`, b.jobID, b.createTime).Scan(&notChecked)
	if err != nil {
		return false, err
	}

	return !notChecked, nil
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

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	err = b.conn.Bus().Notify(buildEventsChannel(b.id))
	if err != nil {
		return false, err
	}

	err = b.conn.Bus().Notify(atc.ComponentBuildTracker)
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
		_, err = tx.Exec(`WITH caches AS (
			SELECT resource_cache_id, build_id
			FROM build_image_resource_caches brc
			JOIN builds b ON b.id = brc.build_id
			WHERE b.job_id = $1
		)
		DELETE FROM build_image_resource_caches birc
		USING caches c
		WHERE c.build_id = birc.build_id AND birc.build_id < $2`,
			b.jobID, b.id)
		if err != nil {
			return err
		}

		rows, err := psql.Select("o.resource_id", "o.version_md5").
			From("build_resource_config_version_outputs o").
			Where(sq.Eq{
				"o.build_id": b.id,
			}).
			RunWith(tx).
			Query()
		if err != nil {
			return err
		}

		defer Close(rows)

		uniqueVersions := map[AlgorithmVersion]bool{}
		outputVersions := map[string][]string{}
		for rows.Next() {
			var resourceID int
			var version string

			err = rows.Scan(&resourceID, &version)
			if err != nil {
				return err
			}

			resourceVersion := AlgorithmVersion{
				ResourceID: resourceID,
				Version:    ResourceVersion(version),
			}

			if !uniqueVersions[resourceVersion] {
				resID := strconv.Itoa(resourceID)
				outputVersions[resID] = append(outputVersions[resID], version)

				uniqueVersions[resourceVersion] = true
			}
		}

		rows, err = psql.Select("i.resource_id", "i.version_md5").
			From("build_resource_config_version_inputs i").
			Where(sq.Eq{
				"i.build_id": b.id,
			}).
			RunWith(tx).
			Query()
		if err != nil {
			return err
		}

		defer Close(rows)

		for rows.Next() {
			var resourceID int
			var version string

			err = rows.Scan(&resourceID, &version)
			if err != nil {
				return err
			}

			resourceVersion := AlgorithmVersion{
				ResourceID: resourceID,
				Version:    ResourceVersion(version),
			}

			if !uniqueVersions[resourceVersion] {
				resID := strconv.Itoa(resourceID)
				outputVersions[resID] = append(outputVersions[resID], version)

				uniqueVersions[resourceVersion] = true
			}
		}

		outputsJSON, err := json.Marshal(outputVersions)
		if err != nil {
			return err
		}

		var rerunOf sql.NullInt64
		if b.rerunOf != 0 {
			rerunOf = sql.NullInt64{Int64: int64(b.rerunOf), Valid: true}
		}

		_, err = psql.Insert("successful_build_outputs").
			Columns("build_id", "job_id", "rerun_of", "outputs").
			Values(b.id, b.jobID, rerunOf, outputsJSON).
			RunWith(tx).
			Exec()
		if err != nil {
			return err
		}

		// recursively archive any child pipelines. This is likely the most common case for
		// automatic archiving so it's worth it to make the feedback more instantenous rather
		// than relying on GC
		pipelineRows, err := pipelinesQuery.
			Prefix(`
WITH RECURSIVE pipelines_to_archive AS (
	SELECT id from pipelines where archived = false AND parent_job_id = $1 AND parent_build_id < $2
	UNION
	SELECT p.id from pipelines p join jobs j on p.parent_job_id = j.id join pipelines_to_archive on j.pipeline_id = pipelines_to_archive.id
)`,
				b.jobID, b.id,
			).
			Where("EXISTS(SELECT 1 FROM pipelines_to_archive pa WHERE pa.id = p.id)").
			RunWith(tx).
			Query()

		if err != nil {
			return err
		}
		defer pipelineRows.Close()

		_, err = archivePipelines(tx, b.conn, b.lockFactory, pipelineRows)
		if err != nil {
			return err
		}
	}

	if b.jobID != 0 {
		err = requestScheduleOnDownstreamJobs(tx, b.jobID)
		if err != nil {
			return err
		}

		err = updateTransitionBuildForJob(tx, b.jobID, b.id, status, b.rerunOf)
		if err != nil {
			return err
		}

		latestNonRerunID, err := latestCompletedNonRerunBuild(tx, b.jobID)
		if err != nil {
			return err
		}

		err = updateLatestCompletedBuildForJob(tx, b.jobID, latestNonRerunID)
		if err != nil {
			return err
		}

		err = updateNextBuildForJob(tx, b.jobID, latestNonRerunID)
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

	config, err := job.Config()
	if err != nil {
		return BuildPreparation{}, false, err
	}

	configInputs := config.Inputs()

	buildInputs, err := job.GetNextBuildInputs()
	if err != nil {
		return BuildPreparation{}, false, err
	}

	resolved := true
	for _, input := range buildInputs {
		if input.ResolveError != "" {
			resolved = false
			break
		}
	}

	inputsSatisfiedStatus := BuildPreparationStatusNotBlocking
	inputs := map[string]BuildPreparationStatus{}
	missingInputReasons := MissingInputReasons{}

	for _, configInput := range configInputs {
		buildInput := BuildInput{}
		found := false
		for _, b := range buildInputs {
			if b.Name == configInput.Name {
				found = true
				buildInput = b
				break
			}
		}

		if found {
			if buildInput.ResolveError == "" {
				if b.IsManuallyTriggered() {
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
				} else {
					inputs[buildInput.Name] = BuildPreparationStatusNotBlocking
				}
			} else {
				inputs[configInput.Name] = BuildPreparationStatusBlocking
				missingInputReasons.RegisterResolveError(configInput.Name, buildInput.ResolveError)
				inputsSatisfiedStatus = BuildPreparationStatusBlocking
			}
		} else {
			if resolved {
				inputs[configInput.Name] = BuildPreparationStatusBlocking
				missingInputReasons.RegisterMissingInput(configInput.Name)
				inputsSatisfiedStatus = BuildPreparationStatusBlocking
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

	resourceConfigScope, err := findOrCreateResourceConfigScope(tx, b.conn, b.lockFactory, resourceConfig, resource, resourceType, resourceTypes)
	if err != nil {
		return err
	}

	newVersion, err := saveResourceVersion(tx, resourceConfigScope.ID(), version, metadata, nil)
	if err != nil {
		return err
	}

	versionBytes, err := json.Marshal(version)
	if err != nil {
		return err
	}

	versionJSON := string(versionBytes)

	if newVersion {
		err = incrementCheckOrder(tx, resourceConfigScope.ID(), versionJSON)
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

	if newVersion {
		err = requestScheduleForJobsUsingResourceConfigScope(tx, resourceConfigScope.ID())
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (b *build) AdoptInputsAndPipes() ([]BuildInput, bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var found bool
	err = psql.Select("inputs_determined").
		From("jobs").
		Where(sq.Eq{
			"id": b.jobID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&found)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	_, err = psql.Delete("build_resource_config_version_inputs").
		Where(sq.Eq{"build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	rows, err := psql.Insert("build_resource_config_version_inputs").
		Columns("resource_id", "version_md5", "name", "first_occurrence", "build_id").
		Select(psql.Select("i.resource_id", "i.version_md5", "i.input_name", "i.first_occurrence").
			Column("?", b.id).
			From("next_build_inputs i").
			Where(sq.Eq{"i.job_id": b.jobID})).
		Suffix("ON CONFLICT (build_id, resource_id, version_md5, name) DO UPDATE SET first_occurrence = EXCLUDED.first_occurrence").
		Suffix("RETURNING name, resource_id, version_md5, first_occurrence").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, false, err
	}

	inputs := InputMapping{}
	for rows.Next() {
		var (
			inputName       string
			firstOccurrence bool
			versionMD5      string
			resourceID      int
		)

		err := rows.Scan(&inputName, &resourceID, &versionMD5, &firstOccurrence)
		if err != nil {
			return nil, false, err
		}

		inputs[inputName] = InputResult{
			Input: &AlgorithmInput{
				AlgorithmVersion: AlgorithmVersion{
					ResourceID: resourceID,
					Version:    ResourceVersion(versionMD5),
				},
				FirstOccurrence: firstOccurrence,
			},
		}
	}

	buildInputs := []BuildInput{}

	for inputName, input := range inputs {
		var versionBlob string

		err = psql.Select("v.version").
			From("resource_config_versions v").
			Join("resources r ON r.resource_config_scope_id = v.resource_config_scope_id").
			Where(sq.Eq{
				"v.version_md5": input.Input.Version,
				"r.id":          input.Input.ResourceID,
			}).
			RunWith(tx).
			QueryRow().
			Scan(&versionBlob)
		if err != nil {
			if err == sql.ErrNoRows {
				tx.Rollback()

				_, err = psql.Update("next_build_inputs").
					Set("resolve_error", fmt.Sprintf("chosen version of input %s not available", inputName)).
					Where(sq.Eq{
						"job_id":     b.jobID,
						"input_name": inputName,
					}).
					RunWith(b.conn).
					Exec()
			}

			return nil, false, err
		}

		var version atc.Version
		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, false, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:            inputName,
			ResourceID:      input.Input.ResourceID,
			Version:         version,
			FirstOccurrence: input.Input.FirstOccurrence,
		})
	}

	_, err = psql.Delete("build_pipes").
		Where(sq.Eq{"to_build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	_, err = psql.Insert("build_pipes").
		Columns("from_build_id", "to_build_id").
		Select(psql.Select("nbp.from_build_id").
			Column("?", b.id).
			From("next_build_pipes nbp").
			Where(sq.Eq{"nbp.to_job_id": b.jobID})).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	_, err = psql.Update("builds").
		Set("inputs_ready", true).
		Where(sq.Eq{
			"id": b.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return buildInputs, true, nil
}

func (b *build) AdoptRerunInputsAndPipes() ([]BuildInput, bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer tx.Rollback()

	var ready bool
	err = psql.Select("inputs_ready").
		From("builds").
		Where(sq.Eq{
			"id": b.rerunOf,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&ready)
	if err != nil {
		return nil, false, err
	}

	if !ready {
		return nil, false, nil
	}

	_, err = psql.Delete("build_resource_config_version_inputs").
		Where(sq.Eq{"build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	rows, err := psql.Insert("build_resource_config_version_inputs").
		Columns("resource_id", "version_md5", "name", "first_occurrence", "build_id").
		Select(psql.Select("i.resource_id", "i.version_md5", "i.name", "false").
			Column("?", b.id).
			From("build_resource_config_version_inputs i").
			Where(sq.Eq{"i.build_id": b.rerunOf})).
		Suffix("ON CONFLICT (build_id, resource_id, version_md5, name) DO NOTHING").
		Suffix("RETURNING name, resource_id, version_md5, first_occurrence").
		RunWith(tx).
		Query()
	if err != nil {
		return nil, false, err
	}

	inputs := InputMapping{}
	for rows.Next() {
		var (
			inputName       string
			firstOccurrence bool
			versionMD5      string
			resourceID      int
		)

		err := rows.Scan(&inputName, &resourceID, &versionMD5, &firstOccurrence)
		if err != nil {
			return nil, false, err
		}

		inputs[inputName] = InputResult{
			Input: &AlgorithmInput{
				AlgorithmVersion: AlgorithmVersion{
					ResourceID: resourceID,
					Version:    ResourceVersion(versionMD5),
				},
				FirstOccurrence: firstOccurrence,
			},
		}
	}

	buildInputs := []BuildInput{}
	for inputName, input := range inputs {
		var versionBlob string

		err = psql.Select("v.version").
			From("resource_config_versions v").
			Join("resources r ON r.resource_config_scope_id = v.resource_config_scope_id").
			Where(sq.Eq{
				"v.version_md5": input.Input.Version,
				"r.id":          input.Input.ResourceID,
			}).
			RunWith(tx).
			QueryRow().
			Scan(&versionBlob)
		if err != nil {
			if err == sql.ErrNoRows {
				tx.Rollback()

				_, err = psql.Update("next_build_inputs").
					Set("resolve_error", fmt.Sprintf("chosen version of input %s not available", inputName)).
					Where(sq.Eq{
						"job_id":     b.jobID,
						"input_name": inputName,
					}).
					RunWith(b.conn).
					Exec()
			}

			return nil, false, err
		}

		var version atc.Version
		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, false, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:            inputName,
			ResourceID:      input.Input.ResourceID,
			Version:         version,
			FirstOccurrence: input.Input.FirstOccurrence,
		})
	}

	_, err = psql.Delete("build_pipes").
		Where(sq.Eq{"to_build_id": b.id}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	_, err = psql.Insert("build_pipes").
		Columns("from_build_id", "to_build_id").
		Select(psql.Select("bp.from_build_id").
			Column("?", b.id).
			From("build_pipes bp").
			Where(sq.Eq{"bp.to_build_id": b.rerunOf})).
		Suffix("ON CONFLICT DO NOTHING").
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	_, err = psql.Update("builds").
		Set("inputs_ready", true).
		Where(sq.Eq{
			"id": b.id,
		}).
		RunWith(tx).
		Exec()
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return buildInputs, true, nil
}

func (b *build) Resources() ([]BuildInput, []BuildOutput, error) {
	inputs := []BuildInput{}
	outputs := []BuildOutput{}

	tx, err := b.conn.Begin()
	if err != nil {
		return nil, nil, err
	}

	defer Rollback(tx)

	rows, err := psql.Select("inputs.name", "resources.id", "versions.version", `COALESCE(inputs.first_occurrence, NOT EXISTS (
			SELECT 1
			FROM build_resource_config_version_inputs i, builds b
			WHERE versions.version_md5 = i.version_md5
			AND resources.resource_config_scope_id = versions.resource_config_scope_id
			AND resources.id = i.resource_id
			AND b.job_id = builds.job_id
			AND i.build_id = b.id
			AND i.build_id < builds.id
		))`).
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
		RunWith(tx).
		Query()
	if err != nil {
		return nil, nil, err
	}

	defer Close(rows)

	for rows.Next() {
		var (
			inputName   string
			firstOcc    bool
			versionBlob string
			version     atc.Version
			resourceID  int
		)

		err = rows.Scan(&inputName, &resourceID, &versionBlob, &firstOcc)
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
			FirstOccurrence: firstOcc,
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
		RunWith(tx).
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

	err = tx.Commit()
	if err != nil {
		return nil, nil, err
	}

	return inputs, outputs, nil
}

func (b *build) SpanContext() propagators.Supplier {
	return b.spanContext
}

func (b *build) SavePipeline(
	pipelineName string,
	teamID int,
	config atc.Config,
	from ConfigVersion,
	initiallyPaused bool,
) (Pipeline, bool, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, false, err
	}

	defer Rollback(tx)

	jobID := newNullInt64(b.jobID)
	buildID := newNullInt64(b.id)
	pipelineID, isNewPipeline, err := savePipeline(tx, pipelineName, config, from, initiallyPaused, teamID, jobID, buildID)
	if err != nil {
		return nil, false, err
	}

	pipeline := newPipeline(b.conn, b.lockFactory)
	err = scanPipeline(
		pipeline,
		pipelinesQuery.
			Where(sq.Eq{"p.id": pipelineID}).
			RunWith(tx).
			QueryRow(),
	)
	if err != nil {
		return nil, false, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, false, err
	}

	return pipeline, isNewPipeline, nil
}

func newNullInt64(i int) sql.NullInt64 {
	return sql.NullInt64{
		Valid: true,
		Int64: int64(i),
	}
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
		jobID, pipelineID, rerunOf, rerunNumber                             sql.NullInt64
		schema, privatePlan, jobName, pipelineName, publicPlan, rerunOfName sql.NullString
		createTime, startTime, endTime, reapTime                            pq.NullTime
		nonce, spanContext                                                  sql.NullString
		drained, aborted, completed                                         bool
		status                                                              string
	)

	err := row.Scan(
		&b.id,
		&b.name,
		&jobID,
		&b.teamID,
		&status,
		&b.isManuallyTriggered,
		&b.scheduled,
		&schema,
		&privatePlan,
		&publicPlan,
		&createTime,
		&startTime,
		&endTime,
		&reapTime,
		&jobName,
		&pipelineID,
		&pipelineName,
		&b.teamName,
		&nonce,
		&drained,
		&aborted,
		&completed,
		&b.inputsReady,
		&rerunOf,
		&rerunOfName,
		&rerunNumber,
		&spanContext,
	)
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
	b.rerunOf = int(rerunOf.Int64)
	b.rerunOfName = rerunOfName.String
	b.rerunNumber = int(rerunNumber.Int64)

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

	if spanContext.Valid {
		err = json.Unmarshal([]byte(spanContext.String), &b.spanContext)
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

	buildVals := make(map[string]interface{})
	for name, value := range vals {
		buildVals[name] = value
	}

	buildVals["needs_v6_migration"] = false

	err := psql.Insert("builds").
		SetMap(buildVals).
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
	return atc.ComponentBuildTracker
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}

func buildAbortChannel(buildID int) string {
	return fmt.Sprintf("build_abort_%d", buildID)
}

func latestCompletedNonRerunBuild(tx Tx, jobID int) (int, error) {
	var latestNonRerunId int
	err := latestCompletedBuildQuery.
		Where(sq.Eq{"job_id": jobID}).
		Where(sq.Eq{"rerun_of": nil}).
		RunWith(tx).
		QueryRow().
		Scan(&latestNonRerunId)
	if err != nil && err == sql.ErrNoRows {
		return 0, nil
	}

	return latestNonRerunId, nil
}

func updateNextBuildForJob(tx Tx, jobID int, latestNonRerunId int) error {
	_, err := tx.Exec(`
		UPDATE jobs AS j
		SET next_build_id = (
			SELECT min(b.id)
			FROM builds b
			INNER JOIN jobs j ON j.id = b.job_id
			WHERE b.job_id = $1
			AND b.status IN ('pending', 'started')
			AND (b.rerun_of IS NULL OR b.rerun_of = $2)
		)
		WHERE j.id = $1
	`, jobID, latestNonRerunId)
	if err != nil {
		return err
	}
	return nil
}

func updateLatestCompletedBuildForJob(tx Tx, jobID int, latestNonRerunId int) error {
	var latestRerunId sql.NullString
	err := latestCompletedBuildQuery.
		Where(sq.Eq{"job_id": jobID}).
		Where(sq.Eq{"rerun_of": latestNonRerunId}).
		RunWith(tx).
		QueryRow().
		Scan(&latestRerunId)
	if err != nil {
		return err
	}

	var id int
	if latestRerunId.Valid {
		id, err = strconv.Atoi(latestRerunId.String)
		if err != nil {
			return err
		}
	} else {
		id = latestNonRerunId
	}

	_, err = tx.Exec(`
		UPDATE jobs AS j
		SET latest_completed_build_id = $1
		WHERE j.id = $2
	`, id, jobID)
	if err != nil {
		return err
	}

	return nil
}

func updateTransitionBuildForJob(tx Tx, jobID int, buildID int, buildStatus BuildStatus, rerunID int) error {
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

	if latestStatus != buildStatus && (isNotRerunBuild(rerunID) || rerunID == latestID) {
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

func isNotRerunBuild(rerunID int) bool {
	return rerunID == 0
}
