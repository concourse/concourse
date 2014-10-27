package resources

import (
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db"
)

type Checker interface {
	CheckResource(config.Resource, db.Version) ([]db.Version, error)
}
