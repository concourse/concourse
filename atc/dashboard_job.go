package atc

import "time"

type Dashboard []DashboardJob

type DashboardJob struct {
	ID           int
	Name         string
	PipelineName string
	TeamName     string
	Paused       bool
	HasNewInputs bool

	FinishedBuild   *DashboardBuild
	NextBuild       *DashboardBuild
	TransitionBuild *DashboardBuild

	Inputs  []DashboardJobInput
	Outputs []JobOutput

	Groups []string
}

type DashboardBuild struct {
	ID           int
	Name         string
	JobName      string
	PipelineName string
	TeamName     string
	Status       string

	StartTime time.Time
	EndTime   time.Time
}

type DashboardJobInput struct {
	Name     string
	Resource string
	Passed   []string
	Trigger  bool
}
