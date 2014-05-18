package db

import (
	"encoding/json"

	"github.com/winston-ci/winston/builds"
)

type DB interface {
	Builds(job string) ([]builds.Build, error)
	CreateBuild(job string) (builds.Build, error)
	GetBuild(job string, id int) (builds.Build, error)

	SaveBuildStatus(job string, build int, status builds.Status) (builds.Build, error)

	BuildLog(job string, build int) ([]byte, error)
	SaveBuildLog(job string, build int, log []byte) error

	GetCurrentSource(resource string) (*json.RawMessage, error)
	SaveCurrentSource(resource string, source *json.RawMessage) error
}
