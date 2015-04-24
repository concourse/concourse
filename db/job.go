package db

type Job struct {
	Name string
}

type SavedJob struct {
	ID           int
	Paused       bool
	PipelineName string
	Job
}
