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

const trackingInterval = 10 * time.Second

//go:generate counterfeiter . BuildDB

type BuildDB interface {
	GetBuild(int) (db.Build, bool, error)
	StartBuild(int, string, string) (bool, error)

	AbortBuild(int) error
	AbortNotifier(int) (db.Notifier, error)

	LeaseBuildTracking(buildID int, interval time.Duration) (db.Lease, bool, error)

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

func (engine *dbEngine) CreateBuild(logger lager.Logger, build db.Build, plan atc.Plan) (Build, error) {
	buildEngine := engine.engines[0]

	createdBuild, err := buildEngine.CreateBuild(logger, build, plan)
	if err != nil {
		return nil, err
	}

	started, err := engine.db.StartBuild(build.ID, buildEngine.Name(), createdBuild.Metadata())
	if err != nil {
		return nil, err
	}

	if !started {
		createdBuild.Abort(logger.Session("aborted-immediately"))
	}

	return &dbBuild{
		id: build.ID,

		engines: engine.engines,

		db: engine.db,
	}, nil
}

func (engine *dbEngine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {
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

func (build *dbBuild) PublicPlan(logger lager.Logger) (atc.PublicBuildPlan, bool, error) {
	model, found, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-get-build-from-database", err)
		return atc.PublicBuildPlan{}, false, err
	}

	if !found || model.Engine == "" {
		return atc.PublicBuildPlan{}, false, nil
	}

	buildEngine, found := build.engines.Lookup(model.Engine)
	if !found {
		logger.Error("unknown-engine", nil, lager.Data{"engine": model.Engine})
		return atc.PublicBuildPlan{}, false, UnknownEngineError{model.Engine}
	}

	engineBuild, err := buildEngine.LookupBuild(logger, model)
	if err != nil {
		return atc.PublicBuildPlan{}, false, err
	}

	return engineBuild.PublicPlan(logger)
}

func (build *dbBuild) Abort(logger lager.Logger) error {
	// the order below is very important to avoid races with build creation.

	lease, leased, err := build.db.LeaseBuildTracking(build.id, trackingInterval)
	if err != nil {
		logger.Error("failed-to-get-lease", err)
		return err
	}

	if !leased {
		// someone else is tracking the build; abort it, which will notify them
		logger.Info("notifying-other-tracker")
		return build.db.AbortBuild(build.id)
	}

	defer lease.Break()

	// no one is tracking the build; abort it ourselves

	// first save the status so that CreateBuild will see a conflict when it
	// tries to mark the build as started.
	err = build.db.AbortBuild(build.id)
	if err != nil {
		logger.Error("failed-to-abort-in-database", err)
		return err
	}

	// reload the model *after* saving the status for the following check to see
	// if it was already started
	model, found, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-get-build-from-database", err)
		return err
	}

	if !found {
		logger.Info("build-not-found")
		return nil
	}

	// if there's an engine, there's a real build to abort
	if model.Engine == "" {
		// otherwise, CreateBuild had not yet tried to start the build, and so it
		// will see the conflict when it tries to transition, and abort itself.
		//
		// finish the build so that the aborted event is put into the event stream
		// even if the build has not started yet
		logger.Info("finishing-build-with-no-engine")
		return build.db.FinishBuild(build.id, db.StatusAborted)
	}

	buildEngine, found := build.engines.Lookup(model.Engine)
	if !found {
		logger.Error("unknown-engine", nil, lager.Data{"engine": model.Engine})
		return UnknownEngineError{model.Engine}
	}

	// find the real build to abort...
	engineBuild, err := buildEngine.LookupBuild(logger, model)
	if err != nil {
		logger.Error("failed-to-lookup-build-in-engine", err)
		return err
	}

	// ...and abort it.
	return engineBuild.Abort(logger)
}

func (build *dbBuild) Resume(logger lager.Logger) {
	lease, leased, err := build.db.LeaseBuildTracking(build.id, trackingInterval)
	if err != nil {
		logger.Error("failed-to-get-lease", err)
		return
	}

	if !leased {
		logger.Debug("build-already-tracked")
		return
	}

	defer lease.Break()

	model, found, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-not-found")
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

	engineBuild, err := buildEngine.LookupBuild(logger, model)
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

			err := engineBuild.Abort(logger)
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

	logger.Info("running")

	engineBuild.Resume(logger)

	doneModel, found, err := build.db.GetBuild(build.id)
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-removed")
		return
	}

	metric.BuildFinished{
		PipelineName:  model.PipelineName,
		JobName:       model.JobName,
		BuildName:     model.Name,
		BuildID:       model.ID,
		BuildStatus:   doneModel.Status,
		BuildDuration: doneModel.EndTime.Sub(doneModel.StartTime),
	}.Emit(logger)
}

func (build *dbBuild) finishWithError(buildID int, logger lager.Logger) {
	err := build.db.FinishBuild(buildID, db.StatusErrored)
	if err != nil {
		logger.Error("failed-to-mark-build-as-errored", err)
	}
}
