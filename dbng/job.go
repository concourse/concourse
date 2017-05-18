package dbng

import (
	"database/sql"
	"encoding/json"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
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

	Reload() (bool, error)

	Pause() error
	Unpause() error

	Builds(page Page) ([]Build, Pagination, error)
	Build(name string) (Build, bool, error)
	FinishedAndNextBuild() (Build, Build, error)
	UpdateFirstLoggedBuildID(newFirstLoggedBuildID int) error
}

var jobsQuery = psql.Select("j.id", "j.name", "j.config", "j.paused", "j.first_logged_build_id", "j.pipeline_id", "p.name", "p.team_id", "t.name", "j.nonce").
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

	conn        Conn
	lockFactory lock.LockFactory
	encryption  EncryptionStrategy
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
	row := buildsQuery.
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
		}).
		// Where(sq.NotEq{"b.status": []BuildStatus{BuildStatusPending, BuildStatusStarted}}).
		Where(sq.Expr("b.status NOT IN ('pending', 'started')")).
		OrderBy("b.id DESC").
		Limit(1).
		RunWith(j.conn).
		QueryRow()

	var finished, next Build

	finishedBuild := &build{conn: j.conn, lockFactory: j.lockFactory, encryption: j.encryption}
	err := scanBuild(finishedBuild, row)
	if err == nil {
		finished = finishedBuild
	} else if err != sql.ErrNoRows {
		return nil, nil, err
	}

	row = buildsQuery.
		Where(sq.Eq{
			"j.name":        j.name,
			"j.pipeline_id": j.pipelineID,
			"b.status":      []BuildStatus{BuildStatusPending, BuildStatusStarted},
		}).
		OrderBy("b.id ASC").
		Limit(1).
		RunWith(j.conn).
		QueryRow()

	nextBuild := &build{conn: j.conn, lockFactory: j.lockFactory, encryption: j.encryption}
	err = scanBuild(nextBuild, row)
	if err == nil {
		next = nextBuild
	} else if err != sql.ErrNoRows {
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (j *job) Builds(page Page) ([]Build, Pagination, error) {
	var (
		err        error
		maxID      int
		minID      int
		firstBuild Build
		lastBuild  Build
		pagination Pagination

		rows *sql.Rows
	)

	query := fmt.Sprintf(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN teams t ON b.team_id = t.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
	`)

	if page.Since == 0 && page.Until == 0 {
		rows, err = j.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY b.id DESC
			LIMIT $3
		`, query), j.name, j.pipelineID, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else if page.Until != 0 {
		rows, err = j.conn.Query(fmt.Sprintf(`
			SELECT sub.*
			FROM (%s
					AND b.id > $3
				ORDER BY b.id ASC
				LIMIT $4
			) sub
			ORDER BY sub.id DESC
		`, query), j.name, j.pipelineID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else {
		rows, err = j.conn.Query(fmt.Sprintf(`
				%s
				AND b.id < $3
			ORDER BY b.id DESC
			LIMIT $4
		`, query), j.name, j.pipelineID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build := &build{conn: j.conn, lockFactory: j.lockFactory, encryption: j.encryption}
		err = scanBuild(build, rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return []Build{}, Pagination{}, nil
	}

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

	firstBuild = builds[0]
	lastBuild = builds[len(builds)-1]

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
	row := buildsQuery.Where(sq.Eq{
		"b.job_id": j.id,
		"b.name":   name,
	}).
		RunWith(j.conn).
		QueryRow()

	build := &build{conn: j.conn, lockFactory: j.lockFactory, encryption: j.encryption}

	err := scanBuild(build, row)
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
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func scanJob(j *job, row scannable) error {
	var (
		configBlob []byte
		nonce      sql.NullString
	)

	err := row.Scan(&j.id, &j.name, &configBlob, &j.paused, &j.firstLoggedBuildID, &j.pipelineID, &j.pipelineName, &j.teamID, &j.teamName, &nonce)
	if err != nil {
		return err
	}

	decryptedConfig, err := j.encryption.Decrypt(string(configBlob), nonce.String)
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

func scanJobs(conn Conn, lockFactory lock.LockFactory, encryption EncryptionStrategy, rows *sql.Rows) ([]Job, error) {
	defer rows.Close()

	jobs := []Job{}

	for rows.Next() {
		job := &job{conn: conn, lockFactory: lockFactory, encryption: encryption}

		err := scanJob(job, rows)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}
