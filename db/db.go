package db

import (
	"time"

	"github.com/concourse/atc"
)

type DB interface {
	GetBuild(buildID int) (Build, error)
	GetAllBuilds() ([]Build, error)
	GetAllStartedBuilds() ([]Build, error)

	GetJobBuild(job string, build string) (Build, error)
	GetAllJobBuilds(job string) ([]Build, error)
	GetCurrentBuild(job string) (Build, error)
	GetJobFinishedAndNextBuild(job string) (*Build, *Build, error)

	GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error)

	CreateJobBuild(job string) (Build, error)
	CreateJobBuildWithInputs(job string, inputs VersionedResources) (Build, error)

	CreateOneOffBuild() (Build, error)

	ScheduleBuild(buildID int, serial bool) (bool, error)
	StartBuild(buildID int, buildGuid, turbineEndpoint string) (bool, error)

	GetLastBuildEventID(buildID int) (int, error)
	GetBuildEvents(buildID int) ([]BuildEvent, error)
	SaveBuildEvent(buildID int, event BuildEvent) error

	SaveBuildInput(buildID int, vr VersionedResource) error
	SaveBuildOutput(buildID int, vr VersionedResource) error

	SaveBuildStatus(buildID int, status Status) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, endTime time.Time) error

	SaveVersionedResource(VersionedResource) error
	GetLatestVersionedResource(resource string) (VersionedResource, error)

	GetLatestInputVersions([]atc.InputConfig) (VersionedResources, error)
	GetJobBuildForInputs(job string, inputs VersionedResources) (Build, error)

	GetNextPendingBuild(job string) (Build, VersionedResources, error)

	GetResourceHistory(resource string) ([]*VersionHistory, error)

	AcquireWriteLockImmediately(locks []NamedLock) (Lock, error)
	AcquireWriteLock(locks []NamedLock) (Lock, error)
	AcquireReadLock(locks []NamedLock) (Lock, error)
	ListLocks() ([]string, error)
}

type ConfigDB interface {
	GetConfig() (atc.Config, error)
	SaveConfig(atc.Config) error
}

type Lock interface {
	Release() error
}

type BuildEvent struct {
	ID      int
	Type    string
	Payload string
}

type BuildInput struct {
	VersionedResource

	FirstOccurrence bool
}

type BuildOutput struct {
	VersionedResource
}

type VersionHistory struct {
	VersionedResource VersionedResource
	InputsTo          []*JobHistory
	OutputsOf         []*JobHistory
}

type JobHistory struct {
	JobName string
	Builds  []Build
}
