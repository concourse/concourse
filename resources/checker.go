package resources

import (
	"github.com/concourse/atc/builds"
	"github.com/concourse/atc/config"
)

type Checker interface {
	CheckResource(config.Resource, builds.Version) ([]builds.Version, error)
}
