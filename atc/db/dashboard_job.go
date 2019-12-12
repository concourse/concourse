package db

import "time"

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

	Inputs []JobInput

	Groups []string
}

type Dashboard []DashboardJob

type DashboardBuild struct {
	ID           int
	Name         string
	JobName      string
	PipelineName string
	TeamName     string
	Status       BuildStatus

	StartTime time.Time
	EndTime   time.Time
}

type JobInput struct {
	Name     string
	Resource string
	Passed   []string
}
