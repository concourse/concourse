package engine

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Engine

type Engine interface {
	Schema() string
	CreateBuild(lager.Logger, db.Build, atc.Plan) (Build, error)
	LookupBuild(lager.Logger, db.Build) (Build, error)
	ReleaseAll(lager.Logger)
}

//go:generate counterfeiter . Build

type Build interface {
	Metadata() string
	Abort(lager.Logger) error
	Resume(lager.Logger)
}

type Engines []Engine

func (engines Engines) Lookup(schema string) (Engine, bool) {
	for _, e := range engines {
		if e.Schema() == schema {
			return e, true
		}
	}

	return nil, false
}
