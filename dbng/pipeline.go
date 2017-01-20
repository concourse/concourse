package dbng

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db/lock"
)

//go:generate counterfeiter . Pipeline

type Pipeline interface {
	ID() int
	SaveJob(job atc.JobConfig) error
	CreateJobBuild(jobName string) (Build, error)
	CreateResource(name string, config atc.ResourceConfig) (*Resource, error)
	AcquireResourceCheckingLock(logger lager.Logger, resource *Resource, length time.Duration, immediate bool) (lock.Lock, bool, error)
	Destroy() error
}

type pipeline struct {
	id     int
	teamID int

	conn        Conn
	lockFactory lock.LockFactory
}

//ConfigVersion is a sequence identifier used for compare-and-swap
type ConfigVersion int

type PipelinePausedState string

const unqualifiedPipelineColumns = "id, name, config, version, paused, team_id, public"

const (
	PipelinePaused   PipelinePausedState = "paused"
	PipelineUnpaused PipelinePausedState = "unpaused"
	PipelineNoChange PipelinePausedState = "nochange"
)

func (state PipelinePausedState) Bool() *bool {
	yes := true
	no := false

	switch state {
	case PipelinePaused:
		return &yes
	case PipelineUnpaused:
		return &no
	case PipelineNoChange:
		return nil
	default:
		panic("unknown pipeline state")
	}
}

func (p *pipeline) ID() int { return p.id }

func (p *pipeline) CreateJobBuild(jobName string) (Build, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	buildName, jobID, err := getNewBuildNameForJob(tx, jobName, p.id)
	if err != nil {
		return nil, err
	}

	var buildID int
	err = psql.Insert("builds").
		Columns("name", "job_id", "team_id", "status", "manually_triggered").
		Values(buildName, jobID, p.teamID, "pending", true).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		return nil, err
	}

	err = createBuildEventSeq(tx, buildID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &build{
		id:         buildID,
		pipelineID: p.id,
		teamID:     p.teamID,
		conn:       p.conn,
	}, nil
}

func (p *pipeline) CreateResource(name string, config atc.ResourceConfig) (*Resource, error) {
	tx, err := p.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback()

	configPayload, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var resourceID int
	err = psql.Insert("resources").
		Columns("pipeline_id", "name", "config", "source_hash").
		Values(p.id, name, configPayload, mapHash(config.Source)).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&resourceID)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &Resource{
		ID:   resourceID,
		Name: name,
	}, nil
}

func (p *pipeline) SaveJob(job atc.JobConfig) error {
	configPayload, err := json.Marshal(job)
	if err != nil {
		return err
	}

	return safeCreateOrUpdate(
		p.conn,
		func(tx Tx) (sql.Result, error) {
			return psql.Insert("jobs").
				Columns("name", "pipeline_id", "config", "active").
				Values(job.Name, p.id, configPayload, true).
				RunWith(tx).
				Exec()
		},
		func(tx Tx) (sql.Result, error) {
			return psql.Update("jobs").
				Set("config", configPayload).
				Set("active", true).
				Where(sq.Eq{
					"name":        job.Name,
					"pipeline_id": p.id,
				}).
				RunWith(tx).
				Exec()
		},
	)
}

func (p *pipeline) Destroy() error {
	tx, err := p.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(fmt.Sprintf(`
		DROP TABLE pipeline_build_events_%d
	`, p.id))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM pipelines WHERE id = $1;
	`, p.id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func getNewBuildNameForJob(tx Tx, jobName string, pipelineID int) (string, int, error) {
	var buildName string
	var jobID int
	err := tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE name = $1 AND pipeline_id = $2
		RETURNING build_number_seq, id
	`, jobName, pipelineID).Scan(&buildName, &jobID)
	return buildName, jobID, err
}
