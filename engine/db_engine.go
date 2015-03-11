package engine

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

var ErrBuildNotActive = errors.New("build not yet active")

//go:generate counterfeiter . BuildDB

type BuildDB interface {
	GetBuild(int) (db.Build, error)
	GetBuildEvents(int, uint) (db.EventSource, error)
	StartBuild(int, string, string) (bool, error)

	AbortBuild(int) error
	AbortNotifier(int) (db.Notifier, error)

	FinishBuild(int, db.Status) error
}

//go:generate counterfeiter . BuildLocker

type BuildLocker interface {
	AcquireWriteLockImmediately([]db.NamedLock) (db.Lock, error)
}

func NewDBEngine(engines Engines, buildDB BuildDB, locker BuildLocker) Engine {
	return &dbEngine{
		engines: engines,

		db:     buildDB,
		locker: locker,
	}
}

type UnknownEngineError struct {
	Engine string
}

func (err UnknownEngineError) Error() string {
	return fmt.Sprintf("unknown build engine: %s", err.Engine)
}

type dbEngine struct {
	engines Engines

	db     BuildDB
	locker BuildLocker
}

func (*dbEngine) Name() string {
	return "db"
}

func (engine *dbEngine) CreateBuild(build db.Build, plan atc.Plan) (Build, error) {
	buildEngine := engine.engines[0]

	createdBuild, err := buildEngine.CreateBuild(build, plan)
	if err != nil {
		return nil, err
	}

	started, err := engine.db.StartBuild(build.ID, buildEngine.Name(), createdBuild.Metadata())
	if err != nil {
		return nil, err
	}

	if !started {
		createdBuild.Abort()
	}

	return &dbBuild{
		id: build.ID,

		engines: engine.engines,

		db:     engine.db,
		locker: engine.locker,
	}, nil
}

func (engine *dbEngine) LookupBuild(build db.Build) (Build, error) {
	return &dbBuild{
		id: build.ID,

		engines: engine.engines,

		db:     engine.db,
		locker: engine.locker,
	}, nil
}

type dbBuild struct {
	id int

	engines Engines

	db     BuildDB
	locker BuildLocker
}

func (build *dbBuild) Metadata() string {
	return strconv.Itoa(build.id)
}

func (build *dbBuild) Abort() error {
	// the order below is very important to avoid races with build creation.

	lock, err := build.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(build.id)})
	if err != nil {
		// someone else is tracking the build; abort it, which will notify them
		return build.db.AbortBuild(build.id)
	}

	defer lock.Release()

	// no one is tracking the build; abort it ourselves

	// first save the status so that CreateBuild will see a conflict when it
	// tries to mark the build as started.
	err = build.db.AbortBuild(build.id)
	if err != nil {
		return err
	}

	// reload the model *after* saving the status for the following check to see
	// if it was already started
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

	buildEngine, found := build.engines.Lookup(model.Engine)
	if !found {
		return UnknownEngineError{model.Engine}
	}

	// find the real build to abort...
	engineBuild, err := buildEngine.LookupBuild(model)
	if err != nil {
		return err
	}

	// ...and abort it.
	return engineBuild.Abort()
}

func (build *dbBuild) Resume(logger lager.Logger) {
	lock, err := build.locker.AcquireWriteLockImmediately([]db.NamedLock{db.BuildTrackingLock(build.id)})
	if err != nil {
		// already being tracked somewhere; short-circuit
		return
	}

	defer lock.Release()

	model, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if model.Engine == "" {
		logger.Error("build-has-no-engine", err)
		return
	}

	buildEngine, found := build.engines.Lookup(model.Engine)
	if !found {
		logger.Error("unknown-build-engine", nil, lager.Data{
			"engine": model.Engine,
		})
		build.finishWithError(model.ID, logger)
		return
	}

	engineBuild, err := buildEngine.LookupBuild(model)
	if err != nil {
		logger.Error("failed-to-lookup-build-from-engine", err)
		build.finishWithError(model.ID, logger)
		return
	}

	aborts, err := build.db.AbortNotifier(build.id)
	if err != nil {
		logger.Error("failed-to-listen-for-aborts", err)
		return
	}

	defer aborts.Close()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-aborts.Notify():
			logger.Info("aborting")

			err := engineBuild.Abort()
			if err != nil {
				logger.Error("failed-to-abort", err)
			}
		case <-done:
		}
	}()

	engineBuild.Resume(logger)
}

func (build *dbBuild) Hijack(target HijackTarget, spec atc.HijackProcessSpec, io HijackProcessIO) (HijackedProcess, error) {
	model, err := build.db.GetBuild(build.id)
	if err != nil {
		return nil, err
	}

	if model.Engine == "" {
		return nil, ErrBuildNotActive
	}

	buildEngine, found := build.engines.Lookup(model.Engine)
	if !found {
		return nil, UnknownEngineError{model.Engine}
	}

	engineBuild, err := buildEngine.LookupBuild(model)
	if err != nil {
		return nil, err
	}

	return engineBuild.Hijack(target, spec, io)
}

func (build *dbBuild) finishWithError(buildID int, logger lager.Logger) {
	err := build.db.FinishBuild(buildID, db.StatusErrored)
	if err != nil {
		logger.Error("failed-to-mark-build-as-errored", err)
	}
}
