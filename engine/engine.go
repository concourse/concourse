package engine

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc/db"
	"github.com/concourse/turbine"
	"github.com/concourse/turbine/event"
	"github.com/pivotal-golang/lager"
)

var ErrBuildNotFound = errors.New("build not found")

type Engine interface {
	CreateBuild(turbine.Build) (Build, error)
	LookupBuild(db.Build) (Build, error)

	ResumeBuild(db.Build, lager.Logger) error
}

type Build interface {
	Guid() string

	Abort() error
	Hijack(garden.ProcessSpec, garden.ProcessIO) error
	Subscribe(from uint) (<-chan event.Event, chan<- struct{}, error)
}
