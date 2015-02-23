package db

type NamedLock interface {
	Name() string
}

type ResourceCheckingLock string

func (resourceCheckingLock ResourceCheckingLock) Name() string {
	return "resourceChecking: " + string(resourceCheckingLock)
}

type JobSchedulingLock string

func (jobSchedulingLock JobSchedulingLock) Name() string {
	return "jobScheduling: " + string(jobSchedulingLock)
}

type BuildTrackingLock string

func (buildTrackingLock BuildTrackingLock) Name() string {
	return "buildTracking: " + string(buildTrackingLock)
}
