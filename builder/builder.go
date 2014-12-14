package builder

import (
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/engine"
	"github.com/concourse/turbine"
)

type BuilderDB interface {
	StartBuild(buildID int, engine, metadata string) (bool, error)
}

type Builder interface {
	Build(db.Build, turbine.Build) error
}

type builder struct {
	db     BuilderDB
	engine engine.Engine
}

func NewBuilder(db BuilderDB, engine engine.Engine) Builder {
	return &builder{
		db:     db,
		engine: engine,
	}
}

func (builder *builder) Build(build db.Build, turbineBuild turbine.Build) error {
	createdBuild, err := builder.engine.CreateBuild(build, turbineBuild)
	if err != nil {
		return err
	}

	started, err := builder.db.StartBuild(build.ID, builder.engine.Name(), createdBuild.Metadata())
	if err != nil {
		return err
	}

	if !started {
		createdBuild.Abort()
	}

	return nil
}
