package db

import (
	"database/sql"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/algorithm"
	"github.com/concourse/atc/db/lock"
)

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

	Reload() (bool, error)

	Pause() error
	Unpause() error

	CreateBuild() (Build, error)
	Builds(page Page) ([]Build, Pagination, error)
	Build(name string) (Build, bool, error)
	FinishedAndNextBuild() (Build, Build, error)
	UpdateFirstLoggedBuildID(newFirstLoggedBuildID int) error
	EnsurePendingBuildExists() error
	GetPendingBuilds() ([]Build, error)

	GetIndependentBuildInputs() ([]BuildInput, error)
	GetNextBuildInputs() ([]BuildInput, bool, error)
	SaveNextInputMapping(inputMapping algorithm.InputMapping) error
	SaveIndependentInputMapping(inputMapping algorithm.InputMapping) error
	DeleteNextInputMapping() error

	SetMaxInFlightReached(bool) error
	GetRunningBuildsBySerialGroup(serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(serialGroups []string) (Build, bool, error)
}

var jobsQuery = psql.Select("j.id", "j.name", "j.config", "j.paused", "j.first_logged_build_id", "j.pipeline_id", "p.name", "p.team_id", "t.name", "j.nonce", "array_to_json(j.tags)").
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

	conn        Conn
	lockFactory lock.LockFactory
}

type Jobs []Job

func (jobs Jobs) Configs() atc.JobConfigs {
	var configs atc.JobConfigs

	for _, j := range jobs {
		configs = append(configs, j.Config())
	}

	return configs
}

func (j *job) ID() int                 { return j.id }
func (j *job) Name() string            { return j.name }
func (j *job) Paused() bool            { return j.paused }
func (j *job) FirstLoggedBuildID() int { return j.firstLoggedBuildID }
func (j *job) PipelineID() int         { return j.pipelineID }
func (j *job) PipelineName() string    { return j.pipelineName }
func (j *job) TeamID() int             { return j.teamID }
func (j *job) TeamName() string        { return j.teamName }
func (j *job) Config() atc.JobConfig   { return j.config }
func (j *job) Tags() []string          { return j.tags }

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
	next, err := j.nextBuild()
	if err != nil {
		return nil, nil, err
	}

	finished, err := j.finishedBuild()
	if err != nil {
		return nil, nil, err
	}

	// query next build again if the build state changed between the two queries
	if next != nil && finished != nil && next.ID() == finished.ID() {
		next = nil

		next, err = j.nextBuild()
		if err != nil {
			return nil, nil, err
		}
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) Builds(page Page) ([]Build, Pagination, error) {
	query := buildsQuery.Where(sq.Eq{"j.id": j.id})

	limit := uint64(page.Limit)

	var reverse bool
	if page.Since == 0 && page.Until == 0 {
		query = query.OrderBy("b.id DESC").Limit(limit)
	} else if page.Until != 0 {
		query = query.Where(sq.Gt{"b.id": page.Until}).OrderBy("b.id ASC").Limit(limit)
		reverse = true
	} else {
		query = query.Where(sq.Lt{"b.id": page.Since}).OrderBy("b.id DESC").Limit(limit)
	}

	rows, err := query.RunWith(j.conn).Query()
	if err != nil {
		return nil, Pagination{}, err
	}

	defer Close(rows)

	builds := []Build{}

	for rows.Next() {
		build := &build{conn: j.conn, lockFactory: j.lockFactory}
		err = scanBuild(build, rows, j.conn.EncryptionStrategy())
		if err != nil {
			return nil, Pagination{}, err
		}

		if reverse {
			builds = append([]Build{build}, builds...)
		} else {
			builds = append(builds, build)
		}
	}

	if len(builds) == 0 {
		return []Build{}, Pagination{}, nil
	}

	var maxID, minID int
	err = psql.Select("COALESCE(MAX(b.id), 0) as maxID", "COALESCE(MIN(b.id), 0) as minID").
		From("builds b").
		Join("jobs j ON b.job_id = j.id").
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		}).
		RunWith(j.conn).
		QueryRow().
		Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	firstBuild := builds[0]
	lastBuild := builds[len(builds)-1]

	var pagination Pagination

	if firstBuild.ID() < maxID {
		pagination.Previous = &Page{
			Until: firstBuild.ID(),
			Limit: page.Limit,
		}
	}

	if lastBuild.ID() > minID {
		pagination.Next = &Page{
			Since: lastBuild.ID(),
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
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

func (j *job) GetNextPendingBuildBySerialGroup(serialGroups []string) (Build, bool, error) {
	err := j.updateSerialGroups(serialGroups)
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
		RunWith(j.conn).
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

func (j *job) GetRunningBuildsBySerialGroup(serialGroups []string) ([]Build, error) {
	err := j.updateSerialGroups(serialGroups)
	if err != nil {
		return nil, err
	}

	rows, err := buildsQuery.Options(`DISTINCT ON (b.id)`).
		Join(`jobs_serial_groups jsg ON j.id = jsg.job_id`).
		Where(sq.Eq{
			"jsg.serial_group": serialGroups,
			"j.pipeline_id":    j.pipelineID,
		}).
		Where(sq.Or{
			sq.Eq{"b.status": BuildStatusStarted},
			sq.Eq{"b.status": BuildStatusPending, "b.scheduled": true},
		}).
		RunWith(j.conn).
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

func (j *job) SetMaxInFlightReached(reached bool) error {
	result, err := psql.Update("jobs").
		Set("max_in_flight_reached", reached).
		Where(sq.Eq{
			"id": j.id,
		}).
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) SaveIndependentInputMapping(inputMapping algorithm.InputMapping) error {
	return j.saveJobInputMapping("independent_build_inputs", inputMapping)
}

func (j *job) SaveNextInputMapping(inputMapping algorithm.InputMapping) error {
	return j.saveJobInputMapping("next_build_inputs", inputMapping)
}

func (j *job) GetIndependentBuildInputs() ([]BuildInput, error) {
	return j.getBuildInputs("independent_build_inputs")
}

func (j *job) GetNextBuildInputs() ([]BuildInput, bool, error) {
	var found bool
	err := psql.Select("inputs_determined").
		From("jobs").
		Where(sq.Eq{
			"name":        j.name,
			"pipeline_id": j.pipelineID,
		}).
		RunWith(j.conn).
		QueryRow().
		Scan(&found)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	// there is a possible race condition where found is true at first but the
	// inputs are deleted by the time we get here
	buildInputs, err := j.getBuildInputs("next_build_inputs")
	return buildInputs, true, err
}

func (j *job) DeleteNextInputMapping() error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Update("jobs").
		Set("inputs_determined", false).
		Where(sq.Eq{
			"name":        j.name,
			"pipeline_id": j.pipelineID,
		}).
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

	return tx.Commit()
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

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	_, err = j.conn.Exec(`REFRESH MATERIALIZED VIEW CONCURRENTLY next_builds_per_job`)
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (j *job) updateSerialGroups(serialGroups []string) error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = psql.Delete("jobs_serial_groups").
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

	return tx.Commit()
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) getBuildInputs(table string) ([]BuildInput, error) {
	rows, err := psql.Select("i.input_name, i.first_occurrence, r.name, v.type, v.version, v.metadata").
		From(table + " i").
		Join("jobs j ON i.job_id = j.id").
		Join("versioned_resources v ON v.id = i.version_id").
		Join("resources r ON r.id = v.resource_id").
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		}).
		RunWith(j.conn).
		Query()
	if err != nil {
		return nil, err
	}

	buildInputs := []BuildInput{}
	for rows.Next() {
		var (
			inputName       string
			firstOccurrence bool
			resourceName    string
			resourceType    string
			versionBlob     string
			metadataBlob    string
			version         ResourceVersion
			metadata        []ResourceMetadataField
		)

		err := rows.Scan(&inputName, &firstOccurrence, &resourceName, &resourceType, &versionBlob, &metadataBlob)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(versionBlob), &version)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(metadataBlob), &metadata)
		if err != nil {
			return nil, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name: inputName,
			VersionedResource: VersionedResource{
				Resource: resourceName,
				Type:     resourceType,
				Version:  version,
				Metadata: metadata,
			},
			FirstOccurrence: firstOccurrence,
		})
	}
	return buildInputs, nil
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

func (j *job) saveJobInputMapping(table string, inputMapping algorithm.InputMapping) error {
	tx, err := j.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	if table == "next_build_inputs" {
		_, err = psql.Update("jobs").
			Set("inputs_determined", true).
			Where(sq.Eq{"id": j.id}).
			Where(sq.Expr("NOT inputs_determined")).
			RunWith(tx).
			Exec()
	}
	if err != nil {
		return err
	}

	rows, err := psql.Select("input_name, version_id, first_occurrence").
		From(table).
		Where(sq.Eq{"job_id": j.id}).
		RunWith(tx).
		Query()
	if err != nil {
		return err
	}

	oldInputMapping := algorithm.InputMapping{}
	for rows.Next() {
		var inputName string
		var inputVersion algorithm.InputVersion
		err = rows.Scan(&inputName, &inputVersion.VersionID, &inputVersion.FirstOccurrence)
		if err != nil {
			return err
		}

		oldInputMapping[inputName] = inputVersion
	}

	for inputName, oldInputVersion := range oldInputMapping {
		inputVersion, found := inputMapping[inputName]
		if !found || inputVersion != oldInputVersion {
			_, err = psql.Delete(table).
				Where(sq.Eq{
					"job_id":     j.id,
					"input_name": inputName,
				}).
				RunWith(tx).
				Exec()
			if err != nil {
				return err
			}
		}
	}

	for inputName, inputVersion := range inputMapping {
		oldInputVersion, found := oldInputMapping[inputName]
		if !found || inputVersion != oldInputVersion {
			_, err := psql.Insert(table).
				SetMap(map[string]interface{}{
					"job_id":           j.id,
					"input_name":       inputName,
					"version_id":       inputVersion.VersionID,
					"first_occurrence": inputVersion.FirstOccurrence,
				}).
				RunWith(tx).
				Exec()
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (j *job) nextBuild() (Build, error) {
	var next Build

	row := buildsQuery.
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
			"b.status":      []BuildStatus{BuildStatusPending, BuildStatusStarted},
		}).
		OrderBy("b.id ASC").
		Limit(1).
		RunWith(j.conn).
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

func (j *job) finishedBuild() (Build, error) {
	var finished Build

	row := buildsQuery.
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		}).
		Where(sq.Expr("b.status NOT IN ('pending', 'started')")).
		OrderBy("b.id DESC").
		Limit(1).
		RunWith(j.conn).
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

func scanJob(j *job, row scannable) error {
	var (
		configBlob []byte
		nonce      sql.NullString
		tagsBlob   []byte
		tags       []string
	)

	err := row.Scan(&j.id, &j.name, &configBlob, &j.paused, &j.firstLoggedBuildID, &j.pipelineID, &j.pipelineName, &j.teamID, &j.teamName, &nonce, &tagsBlob)
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

	json.Unmarshal(tagsBlob, &tags)
	j.tags = tags

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
