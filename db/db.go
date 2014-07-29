package db

import (
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type DB interface {
	RegisterJob(name string) error
	RegisterResource(name string) error

	Builds(job string) ([]builds.Build, error)
	GetBuild(job string, id int) (builds.Build, error)
	GetCurrentBuild(job string) (builds.Build, error)

	GetBuildResources(job string, id int) (inputs, outputs builds.VersionedResources, err error)

	CreateBuild(job string) (builds.Build, error)
	ScheduleBuild(job string, id int, serial bool) (bool, error)
	StartBuild(job string, id int, abortURL string) (bool, error)
	AbortBuild(job string, id int) error

	SaveBuildStatus(job string, build int, status builds.Status) error

	BuildLog(job string, build int) ([]byte, error)
	AppendBuildLog(job string, build int, log []byte) error

	SaveBuildInput(job string, build int, vr builds.VersionedResource) error
	SaveBuildOutput(job string, build int, vr builds.VersionedResource) error

	SaveVersionedResource(builds.VersionedResource) error
	GetLatestVersionedResource(name string) (builds.VersionedResource, error)

	GetLatestInputVersions([]config.Input) (builds.VersionedResources, error)
	GetBuildForInputs(job string, inputs builds.VersionedResources) (builds.Build, error)
	CreateBuildWithInputs(job string, inputs builds.VersionedResources) (builds.Build, error)

	GetNextPendingBuild(job string) (builds.Build, builds.VersionedResources, error)

	GetResourceHistory(resource string) ([]*VersionHistory, error)
}

type VersionHistory struct {
	VersionedResource builds.VersionedResource
	Jobs              []*JobHistory
}

type JobHistory struct {
	JobName string
	Builds  []builds.Build
}
