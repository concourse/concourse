package engine

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
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

	LeaseTrack(buildID int, interval time.Duration) (db.Lease, bool, error)

	FinishBuild(int, db.Status) error
}

func NewDBEngine(engines Engines, buildDB BuildDB) Engine {
	return &dbEngine{
		engines: engines,

		db: buildDB,
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

	db BuildDB
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

		db: engine.db,
	}, nil
}

func (engine *dbEngine) LookupBuild(build db.Build) (Build, error) {
	return &dbBuild{
		id: build.ID,

		engines: engine.engines,

		db: engine.db,
	}, nil
}

type dbBuild struct {
	id int

	engines Engines

	db BuildDB
}

func (build *dbBuild) Metadata() string {
	return strconv.Itoa(build.id)
}

func (build *dbBuild) Abort() error {
	// the order below is very important to avoid races with build creation.

	lease, leased, err := build.db.LeaseTrack(build.id, time.Minute)

	if err != nil {
		return err
	}

	if !leased {
		// someone else is tracking the build; abort it, which will notify them
		return build.db.AbortBuild(build.id)
	}

	defer lease.Break()

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
		//
		// finish the build so that the aborted event is put into the event stream
		// even if the build has not started yet
		return build.db.FinishBuild(build.id, db.StatusAborted)
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
	lease, leased, err := build.db.LeaseTrack(build.id, time.Minute)

	if err != nil {
		logger.Error("failed-to-get-lease", err)
		return
	}

	if !leased {
		// already being tracked somewhere; short-circuit
		return
	}

	defer lease.Break()

	model, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if model.Engine == "" {
		logger.Error("build-has-no-engine", err)
		return
	}

	if !model.IsRunning() {
		logger.Info("build-already-finished", lager.Data{
			"build-id": build.id,
		})
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

	metric.BuildStarted{
		PipelineName: model.PipelineName,
		JobName:      model.JobName,
		BuildName:    model.Name,
		BuildID:      model.ID,
	}.Emit(logger)

	engineBuild.Resume(logger)

	metric.BuildFinished{
		PipelineName: model.PipelineName,
		JobName:      model.JobName,
		BuildName:    model.Name,
		BuildID:      model.ID,
		Duration:     time.Since(model.StartTime),
	}.Emit(logger)
}

func (build *dbBuild) finishWithError(buildID int, logger lager.Logger) {
	err := build.db.FinishBuild(buildID, db.StatusErrored)
	if err != nil {
		logger.Error("failed-to-mark-build-as-errored", err)
	}
}
