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

//go:generate counterfeiter . Engine
type Engine interface {
	Name() string

	CreateBuild(db.Build, turbine.Build) (Build, error)
	LookupBuild(db.Build) (Build, error)
}

//go:generate counterfeiter . Build
type Build interface {
	Metadata() string

	Abort() error
	Hijack(garden.ProcessSpec, garden.ProcessIO) error
	Subscribe(from uint) (<-chan event.Event, chan<- struct{}, error)
	Resume(lager.Logger) error
}
