package dbng

import (
	"encoding/json"

	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/atc"
)

//go:generate counterfeiter . Job

type Job interface {
	ID() int
	Name() string
	Paused() bool
	FirstLoggedBuildID() string
	PipelineID() int
	PipelineName() string
	TeamID() int
	TeamName() string
	Config() atc.JobConfig
}

var jobsQuery = psql.Select("j.id", "j.name", "j.config", "j.paused", "j.first_logged_build_id", "j.pipeline_id", "p.name", "p.team_id", "t.name").
	From("jobs j, pipelines p").
	LeftJoin("teams t ON p.team_id = t.id").
	Where(sq.Expr("j.pipeline_id = p.id"))

type job struct {
	id                 int
	name               string
	paused             bool
	firstLoggedBuildID string
	pipelineID         int
	pipelineName       string
	teamID             int
	teamName           string
	config             atc.JobConfig

	conn Conn
}

func (j *job) ID() int                    { return j.id }
func (j *job) Name() string               { return j.name }
func (j *job) Paused() bool               { return j.paused }
func (j *job) FirstLoggedBuildID() string { return j.firstLoggedBuildID }
func (j *job) PipelineID() int            { return j.pipelineID }
func (j *job) PipelineName() string       { return j.pipelineName }
func (j *job) TeamID() int                { return j.teamID }
func (j *job) TeamName() string           { return j.teamName }
func (j *job) Config() atc.JobConfig      { return j.config }

func scanJob(j *job, row scannable) error {
	var configBlob []byte

	err := row.Scan(&j.id, &j.name, &configBlob, &j.paused, &j.firstLoggedBuildID, &j.pipelineID, &j.pipelineName, &j.teamID, &j.teamName)
	if err != nil {
		return err
	}

	var config atc.JobConfig
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return err
	}
	j.config = config

	return nil
}
