package db

type usedTaskCache struct {
	id       int
	jobID    int
	stepName string
	path     string
}

type UsedTaskCache interface {
	ID() int

	JobID() int
	StepName() string
	Path() string
}

func (tc *usedTaskCache) ID() int          { return tc.id }
func (tc *usedTaskCache) JobID() int       { return tc.jobID }
func (tc *usedTaskCache) StepName() string { return tc.stepName }
func (tc *usedTaskCache) Path() string     { return tc.path }
