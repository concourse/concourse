package engine

import (
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . Engine

type Engine interface {
	Name() string

	CreateBuild(lager.Logger, db.Build, atc.Plan) (Build, error)
	LookupBuild(lager.Logger, db.Build) (Build, error)
}

//go:generate counterfeiter . Build

type Build interface {
	Metadata() string

	PublicPlan(lager.Logger) (atc.PublicBuildPlan, error)

	Abort(lager.Logger) error
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
