package db

import (
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type DB interface {
	Builds(job string) ([]builds.Build, error)
	CreateBuild(job string) (builds.Build, error)
	GetBuild(job string, id int) (builds.Build, error)
	GetCurrentBuild(job string) (builds.Build, error)

	SaveBuildInput(job string, build int, input builds.Input) error
	SaveBuildStatus(job string, build int, status builds.Status) error

	BuildLog(job string, build int) ([]byte, error)
	SaveBuildLog(job string, build int, log []byte) error

	GetCurrentSource(job, input string) (config.Source, error)
	SaveCurrentSource(job, input string, source config.Source) error

	GetCommonOutputs(jobs []string, resourceName string) ([]config.Source, error)
	SaveOutputSource(job string, build int, resourceName string, source config.Source) error
}
