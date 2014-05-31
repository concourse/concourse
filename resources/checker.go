package resources

import (
	"github.com/winston-ci/winston/builds"
	"github.com/winston-ci/winston/config"
)

type Checker interface {
	CheckResource(config.Resource, builds.Version) []builds.Version
}
