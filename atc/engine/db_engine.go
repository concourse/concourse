package engine

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/metric"
)

const trackLockDuration = time.Minute

func NewDBEngine(engines Engines) Engine {
	return &dbEngine{
		engines:   engines,
		releaseCh: make(chan struct{}),
		waitGroup: new(sync.WaitGroup),
	}
}

type UnknownEngineError struct {
	Schema string
}

func (err UnknownEngineError) Error() string {
	return fmt.Sprintf("unknown build engine schema: %s", err.Schema)
}

type dbEngine struct {
	engines   Engines
	releaseCh chan struct{}
	waitGroup *sync.WaitGroup
}

func (*dbEngine) Schema() string {
	return "db"
}

func (engine *dbEngine) CreateBuild(logger lager.Logger, build db.Build, plan atc.Plan) (Build, error) {
	buildEngine := engine.engines[0]

	started, err := build.Start(buildEngine.Schema(), plan)
	if err != nil {
		return nil, err
	}

	if !started {
		if err = build.Finish(db.BuildStatusAborted); err != nil {
			return nil, err
		}
		return nil, errors.New("Build not started, marking as aborted")
	}

	return &dbBuild{
		engines:   engine.engines,
		releaseCh: engine.releaseCh,
		waitGroup: engine.waitGroup,
		build:     build,
	}, nil
}

func (engine *dbEngine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {
	return &dbBuild{
		engines:   engine.engines,
		releaseCh: engine.releaseCh,
		waitGroup: engine.waitGroup,
		build:     build,
	}, nil
}

func (engine *dbEngine) ReleaseAll(logger lager.Logger) {
	logger.Info("calling-release-on-builds")

	close(engine.releaseCh)

	logger.Info("waiting-on-builds")

	for _, e := range engine.engines {
		e.ReleaseAll(logger)
	}

	engine.waitGroup.Wait()

	logger.Info("finished-waiting-on-builds")
}

type dbBuild struct {
	engines   Engines
	releaseCh chan struct{}
	build     db.Build
	waitGroup *sync.WaitGroup
}

func (build *dbBuild) Metadata() string {
	return strconv.Itoa(build.build.ID())
}

func (build *dbBuild) Abort(logger lager.Logger) error {
	// the order below is very important to avoid races with build creation.

	lock, acquired, err := build.build.AcquireTrackingLock(logger, trackLockDuration)
	if err != nil {
		logger.Error("failed-to-get-lock", err)
		return err
	}

	if !acquired {
		// someone else is tracking the build; abort it, which will notify them
		logger.Info("notifying-other-tracker")
		return build.build.MarkAsAborted()
	}

	defer lock.Release()

	// no one is tracking the build; abort it ourselves

	// first save the status so that CreateBuild will see a conflict when it
	// tries to mark the build as started.
	err = build.build.MarkAsAborted()
	if err != nil {
		logger.Error("failed-to-abort-in-database", err)
		return err
	}

	// reload the model *after* saving the status for the following check to see
	// if it was already started
	found, err := build.build.Reload()
	if err != nil {
		logger.Error("failed-to-get-build-from-database", err)
		return err
	}

	if !found {
		logger.Info("build-not-found")
		return nil
	}

	schema := build.build.Schema()
	// if there's an schema, there's a real build to abort
	if schema == "" {
		// otherwise, CreateBuild had not yet tried to start the build, and so it
		// will see the conflict when it tries to transition, and abort itself.
		//
		// finish the build so that the aborted event is put into the event stream
		// even if the build has not started yet
		logger.Info("finishing-build-with-no-engine")
		return build.build.Finish(db.BuildStatusAborted)
	}

	buildEngine, found := build.engines.Lookup(schema)
	if !found {
		err := UnknownEngineError{schema}
		logger.Error("unknown-engine", err, lager.Data{"schema": schema})
		return err
	}

	// find the real build to abort...
	engineBuild, err := buildEngine.LookupBuild(logger, build.build)
	if err != nil {
		logger.Error("failed-to-lookup-build-in-engine", err)
		return err
	}

	// ...and abort it.
	return engineBuild.Abort(logger)
}

func (build *dbBuild) Resume(logger lager.Logger) {
	build.waitGroup.Add(1)
	defer build.waitGroup.Done()

	lock, acquired, err := build.build.AcquireTrackingLock(logger, trackLockDuration)
	if err != nil {
		logger.Error("failed-to-get-lock", err)
		return
	}

	if !acquired {
		logger.Debug("build-already-tracked")
		return
	}

	defer lock.Release()

	found, err := build.build.Reload()
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-not-found")
		return
	}

	schema := build.build.Schema()
	if schema == "" {
		logger.Error("build-has-no-schema", err)
		return
	}

	if !build.build.IsRunning() {
		logger.Info("build-already-finished", lager.Data{
			"build-id": build.build.ID(),
		})
		return
	}

	buildEngine, found := build.engines.Lookup(schema)
	if !found {
		err := UnknownEngineError{schema}
		logger.Error("unknown-engine", err, lager.Data{
			"schema": schema,
		})
		build.finishWithError(logger, err)
		return
	}

	engineBuild, err := buildEngine.LookupBuild(logger, build.build)
	if err != nil {
		logger.Error("failed-to-lookup-build-from-engine", err)
		build.finishWithError(logger, err)
		return
	}

	aborts, err := build.build.AbortNotifier()
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
		case <-build.releaseCh:
			logger.Info("releasing")
		case <-done:
		}
	}()

	metric.BuildStarted{
		PipelineName: build.build.PipelineName(),
		JobName:      build.build.JobName(),
		BuildName:    build.build.Name(),
		BuildID:      build.build.ID(),
		TeamName:     build.build.TeamName(),
	}.Emit(logger)

	logger.Info("running", lager.Data{
		"build":    build.build.ID(),
		"pipeline": build.build.PipelineName(),
		"job":      build.build.JobName(),
	})
	engineBuild.Resume(logger)

	found, err = build.build.Reload()
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-removed")
		return
	}

	if !build.build.IsRunning() {
		metric.BuildFinished{
			PipelineName:  build.build.PipelineName(),
			JobName:       build.build.JobName(),
			BuildName:     build.build.Name(),
			BuildID:       build.build.ID(),
			BuildStatus:   build.build.Status(),
			BuildDuration: build.build.EndTime().Sub(build.build.StartTime()),
			TeamName:      build.build.TeamName(),
		}.Emit(logger)
	}
}

func (build *dbBuild) finishWithError(logger lager.Logger, finishErr error) {
	err := build.build.FinishWithError(finishErr)
	if err != nil {
		logger.Error("failed-to-mark-build-as-errored", err)
	}
}
