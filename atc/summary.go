package atc

import "time"

type JobSummary struct {
	ID                   int
	Name                 string
	PipelineID           int
	PipelineName         string
	PipelineInstanceVars InstanceVars
	TeamName             string
	Paused               bool
	HasNewInputs         bool

	FinishedBuild   *BuildSummary
	NextBuild       *BuildSummary
	TransitionBuild *BuildSummary

	Inputs  []JobInputSummary
	Outputs []JobOutput

	Groups []string
}

type BuildSummary struct {
	ID                   int
	Name                 string
	JobName              string
	PipelineID           int
	PipelineName         string
	PipelineInstanceVars InstanceVars
	TeamName             string
	Status               string

	StartTime time.Time
	EndTime   time.Time
}

type JobInputSummary struct {
	Name     string
	Resource string
	Passed   []string
	Trigger  bool
}
