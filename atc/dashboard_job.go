package atc

import "time"

type Dashboard []DashboardJob

type DashboardJob struct {
	ID                   int
	Name                 string
	PipelineID           int
	PipelineName         string
	PipelineInstanceVars InstanceVars
	TeamName             string
	Paused               bool
	HasNewInputs         bool

	FinishedBuild   *DashboardBuild
	NextBuild       *DashboardBuild
	TransitionBuild *DashboardBuild

	Inputs  []DashboardJobInput
	Outputs []JobOutput

	Groups []string
}

type DashboardBuild struct {
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

type DashboardJobInput struct {
	Name     string
	Resource string
	Passed   []string
	Trigger  bool
}
