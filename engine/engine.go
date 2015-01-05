package engine

import (
	"errors"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

var ErrBuildNotFound = errors.New("build not found")

//go:generate counterfeiter . Engine
type Engine interface {
	Name() string

	CreateBuild(db.Build, atc.BuildPlan) (Build, error)
	LookupBuild(db.Build) (Build, error)
}

//go:generate counterfeiter . Build
type Build interface {
	Metadata() string

	Abort() error
	Hijack(garden.ProcessSpec, garden.ProcessIO) (garden.Process, error)
	Resume(lager.Logger)
}

type Engines []Engine

func (engines Engines) Lookup(name string) (Engine, bool) {
	for _, e := range engines {
		if e.Name() == name {
			return e, true
		}
	}

	return nil, false
}
