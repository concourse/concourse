package db

import "github.com/concourse/atc/builds"

type DB interface {
	RegisterJob(name string) error
	RegisterResource(name string) error

	Builds(job string) ([]builds.Build, error)
	GetBuild(job string, id int) (builds.Build, error)
	GetCurrentBuild(job string) (builds.Build, error)

	AttemptBuild(job string, input string, version builds.Version, serial bool) (builds.Build, error)
	CreateBuild(job string) (builds.Build, error)
	ScheduleBuild(job string, id int, serial bool) (bool, error)
	StartBuild(job string, id int, abortURL string) (bool, error)
	AbortBuild(job string, id int) error

	SaveBuildStatus(job string, build int, status builds.Status) error

	BuildLog(job string, build int) ([]byte, error)
	AppendBuildLog(job string, build int, log []byte) error

	GetCurrentVersion(job, input string) (builds.Version, error)
	SaveCurrentVersion(job, input string, source builds.Version) error

	SaveBuildInput(job string, build int, vr builds.VersionedResource) error
	SaveBuildOutput(job string, build int, vr builds.VersionedResource) error

	SaveVersionedResource(builds.VersionedResource) error
	GetLatestVersionedResource(name string) (builds.VersionedResource, error)

	GetCommonOutputs(jobs []string, resourceName string) ([]builds.Version, error)
}
