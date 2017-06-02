package db

type DashboardJob struct {
	Job Job

	FinishedBuild Build
	NextBuild     Build
}

type Dashboard []DashboardJob
