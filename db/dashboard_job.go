package db

type DashboardJob struct {
	Job Job

	FinishedBuild   Build
	NextBuild       Build
	TransitionBuild Build
}

type Dashboard []DashboardJob
