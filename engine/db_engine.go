package engine

import (
	"errors"
	"strconv"

	garden "github.com/cloudfoundry-incubator/garden/api"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

var ErrBuildNotActive = errors.New("build not yet active")

//go:generate counterfeiter . BuildDB
type BuildDB interface {
	GetBuild(int) (db.Build, error)
	StartBuild(int, string, string) (bool, error)

	SaveBuildStatus(int, db.Status) error
}

//go:generate counterfeiter . BuildLocker
type BuildLocker interface {
	AcquireWriteLockImmediately([]db.NamedLock) (db.Lock, error)
}

func NewDBEngine(engine Engine, buildDB BuildDB, locker BuildLocker) Engine {
	return &dbEngine{
		engine: engine,

		db:     buildDB,
		locker: locker,
	}
}

type dbEngine struct {
	engine Engine

	db     BuildDB
	locker BuildLocker
}

func (*dbEngine) Name() string {
	return "db"
}

func (engine *dbEngine) CreateBuild(build db.Build, plan atc.BuildPlan) (Build, error) {
	createdBuild, err := engine.engine.CreateBuild(build, plan)
	if err != nil {
		return nil, err
	}

	started, err := engine.db.StartBuild(build.ID, engine.engine.Name(), createdBuild.Metadata())
	if err != nil {
		return nil, err
	}

	if !started {
		createdBuild.Abort()
	}

	return &dbBuild{
		id: build.ID,

		engine: engine.engine,

		db:     engine.db,
		locker: engine.locker,
	}, nil
}

func (engine *dbEngine) LookupBuild(build db.Build) (Build, error) {
	return &dbBuild{
		id: build.ID,

		engine: engine.engine,

		db:     engine.db,
		locker: engine.locker,
	}, nil
}

type dbBuild struct {
	id int

	engine Engine

	db     BuildDB
	locker BuildLocker
}

func (build *dbBuild) Metadata() string {
	return strconv.Itoa(build.id)
}

func (build *dbBuild) Abort() error {
	// the order below is very important to avoid races with build creation.

	// first save the status so that CreateBuild will see a conflict when it
	// tries to mark the build as started.
	err := build.db.SaveBuildStatus(build.id, db.StatusAborted)
	if err != nil {
		return err
	}

	// reload the model *after* saving the status for the following check
	model, err := build.db.GetBuild(build.id)
	if err != nil {
		return err
	}

	// if there's an engine, there's a real build to abort
	if model.Engine == "" {
		// otherwise, CreateBuild had not yet tried to start the build, and so it
		// will see the conflict when it tries to transition, and abort itself.
		return nil
	}

	// find the real build to abort...
	engineBuild, err := build.engine.LookupBuild(model)
	if err != nil {
		return err
	}

	// ...and abort it.
	return engineBuild.Abort()
}

func (build *dbBuild) Hijack(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	model, err := build.db.GetBuild(build.id)
	if err != nil {
		return nil, err
	}

	if model.Engine == "" {
		return nil, ErrBuildNotActive
	}

	engineBuild, err := build.engine.LookupBuild(model)
	if err != nil {
		return nil, err
	}

	return engineBuild.Hijack(spec, io)
}

func (build *dbBuild) Subscribe(from uint) (EventSource, error) {
	model, err := build.db.GetBuild(build.id)
	if err != nil {
		return nil, err
	}

	if model.Engine == "" {
		return nil, ErrBuildNotActive
	}

	engineBuild, err := build.engine.LookupBuild(model)
	if err != nil {
		return nil, err
	}

	return engineBuild.Subscribe(from)
}

func (build *dbBuild) Resume(logger lager.Logger) error {
	model, err := build.db.GetBuild(build.id)
	if err != nil {
		return err
	}

	if model.Engine == "" {
		return nil
	}

	engineBuild, err := build.engine.LookupBuild(model)
	if err != nil {
		return err
	}

	lock, err := build.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(build.id)})
	if err != nil {
		return nil
	}

	defer lock.Release()

	return engineBuild.Resume(logger)
}
