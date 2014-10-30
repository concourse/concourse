package db

type NamedLock interface {
	Name() string
}

type ResourceLock string

func (resourceLock ResourceLock) Name() string {
	return "resource: " + string(resourceLock)
}

type JobSchedulingLock string

func (jobSchedulingLock JobSchedulingLock) Name() string {
	return "jobScheduling: " + string(jobSchedulingLock)
}
