package db

import "github.com/winston-ci/winston/builds"

type DB interface {
	Builds(job string) ([]builds.Build, error)
	CreateBuild(job string) (builds.Build, error)
	GetBuild(job string, id int) (builds.Build, error)
	GetCurrentBuild(job string) (builds.Build, error)

	SaveBuildInput(job string, build int, input builds.Input) error
	SaveBuildStatus(job string, build int, status builds.Status) error

	BuildLog(job string, build int) ([]byte, error)
	SaveBuildLog(job string, build int, log []byte) error

	GetCurrentVersion(job, input string) (builds.Version, error)
	SaveCurrentVersion(job, input string, source builds.Version) error

	GetCommonOutputs(jobs []string, resourceName string) ([]builds.Version, error)
	SaveOutputVersion(job string, build int, resourceName string, version builds.Version) error
}
