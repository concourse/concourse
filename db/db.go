package db

import (
	"time"

	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type DB interface {
	RegisterJob(name string) error
	RegisterResource(name string) error

	GetBuild(buildID int) (builds.Build, error)
	GetAllBuilds() ([]builds.Build, error)

	GetJobBuild(job string, build string) (builds.Build, error)
	GetAllJobBuilds(job string) ([]builds.Build, error)
	GetCurrentBuild(job string) (builds.Build, error)
	GetJobFinishedAndNextBuild(job string) (*builds.Build, *builds.Build, error)

	GetBuildResources(buildID int) ([]BuildInput, []BuildOutput, error)

	CreateJobBuild(job string) (builds.Build, error)
	CreateJobBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error)

	CreateOneOffBuild() (builds.Build, error)

	ScheduleBuild(buildID int, serial bool) (bool, error)
	StartBuild(buildID int, abortURL, hijackURL string) (bool, error)

	BuildLog(buildID int) ([]byte, error)
	AppendBuildLog(buildID int, log []byte) error

	SaveBuildInput(buildID int, vr builds.VersionedResource) error
	SaveBuildOutput(buildID int, vr builds.VersionedResource) error

	AbortBuild(buildID int) (string, error)
	SaveBuildStatus(buildID int, status builds.Status) error

	SaveBuildStartTime(buildID int, startTime time.Time) error
	SaveBuildEndTime(buildID int, endTime time.Time) error

	SaveVersionedResource(builds.VersionedResource) error
	GetLatestVersionedResource(resource string) (builds.VersionedResource, error)

	GetLatestInputVersions([]config.Input) (builds.VersionedResources, error)
	GetJobBuildForInputs(job string, inputs builds.VersionedResources) (builds.Build, error)

	GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error)

	GetResourceHistory(resource string) ([]*VersionHistory, error)

	AcquireResourceCheckingLock() (Lock, error)
}

type Lock interface {
	Release() error
}

type BuildInput struct {
	builds.VersionedResource

	FirstOccurrence bool
}

type BuildOutput struct {
	builds.VersionedResource
}

type VersionHistory struct {
	VersionedResource builds.VersionedResource
	InputsTo          []*JobHistory
	OutputsOf         []*JobHistory
}

type JobHistory struct {
	JobName string
	Builds  []builds.Build
}
