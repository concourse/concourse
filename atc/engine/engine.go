package engine

import (
	"context"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
)

//go:generate counterfeiter . Engine

type Engine interface {
	LookupBuild(lager.Logger, db.Build) (Build, error)
	ReleaseAll(lager.Logger)
}

//go:generate counterfeiter . Build

type Build interface {
	Resume(lager.Logger)
}

//go:generate counterfeiter . StepBuilder

type StepBuilder interface {
	BuildStep(db.Build) (exec.Step, error)
}

func NewEngine(builder StepBuilder) Engine {
	return &engine{
		builder: builder,

		release:       make(chan bool),
		trackedStates: new(sync.Map),
		waitGroup:     new(sync.WaitGroup),
	}
}

type engine struct {
	builder StepBuilder

	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup
}

func (engine *engine) LookupBuild(logger lager.Logger, build db.Build) (Build, error) {

	ctx, cancel := context.WithCancel(context.Background())

	return NewBuild(
		ctx,
		cancel,
		build,
		engine.builder,
		engine.release,
		engine.trackedStates,
		engine.waitGroup,
	), nil
}

func (engine *engine) ReleaseAll(logger lager.Logger) {
	logger.Info("calling-release-on-builds")

	close(engine.release)

	logger.Info("waiting-on-builds")

	engine.waitGroup.Wait()

	logger.Info("finished-waiting-on-builds")
}

func NewBuild(
	ctx context.Context,
	cancel func(),
	build db.Build,
	builder StepBuilder,
	release chan bool,
	trackedStates *sync.Map,
	waitGroup *sync.WaitGroup,
) Build {
	return &execBuild{
		ctx:    ctx,
		cancel: cancel,

		build:   build,
		builder: builder,

		release:       release,
		trackedStates: trackedStates,
		waitGroup:     waitGroup,
	}
}

type execBuild struct {
	ctx    context.Context
	cancel func()

	build   db.Build
	builder StepBuilder

	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup
}

func (build *execBuild) Resume(logger lager.Logger) {
	build.waitGroup.Add(1)
	defer build.waitGroup.Done()

	logger = logger.WithData(lager.Data{
		"build":    build.build.ID(),
		"pipeline": build.build.PipelineName(),
		"job":      build.build.JobName(),
	})

	lock, acquired, err := build.build.AcquireTrackingLock(logger, time.Minute)
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

	if !build.build.IsRunning() {
		logger.Info("build-already-finished")
		return
	}

	notifier, err := build.build.AbortNotifier()
	if err != nil {
		logger.Error("failed-to-listen-for-aborts", err)
		return
	}

	defer notifier.Close()

	step, err := build.builder.BuildStep(build.build)
	if err != nil {
		logger.Error("failed-to-build-step", err)
		return
	}

	build.trackStarted(logger)
	defer build.trackFinished(logger)

	logger.Info("running")

	state := build.runState()
	defer build.clearRunState()

	noleak := make(chan bool)
	defer close(noleak)

	go func() {
		select {
		case <-noleak:
		case <-notifier.Notify():
			logger.Info("aborting")
			build.cancel()
		}
	}()

	done := make(chan error)
	go func() {
		ctx := lagerctx.NewContext(build.ctx, logger)
		done <- step.Run(ctx, state)
	}()

	select {
	case <-build.release:
		logger.Info("releasing")

	case err = <-done:
		build.finish(logger.Session("finish"), err, step.Succeeded())
	}
}

func (build *execBuild) finish(logger lager.Logger, err error, succeeded bool) {
	if err == context.Canceled {
		build.saveStatus(logger, atc.StatusAborted)
		logger.Info("aborted")

	} else if err != nil {
		build.saveStatus(logger, atc.StatusErrored)
		logger.Info("errored", lager.Data{"error": err.Error()})

	} else if succeeded {
		build.saveStatus(logger, atc.StatusSucceeded)
		logger.Info("succeeded")

	} else {
		build.saveStatus(logger, atc.StatusFailed)
		logger.Info("failed")
	}
}

func (build *execBuild) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	if err := build.build.Finish(db.BuildStatus(status)); err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (build *execBuild) trackStarted(logger lager.Logger) {
	metric.BuildStarted{
		PipelineName: build.build.PipelineName(),
		JobName:      build.build.JobName(),
		BuildName:    build.build.Name(),
		BuildID:      build.build.ID(),
		TeamName:     build.build.TeamName(),
	}.Emit(logger)
}

func (build *execBuild) trackFinished(logger lager.Logger) {
	found, err := build.build.Reload()
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

func (build *execBuild) runState() exec.RunState {
	existingState, _ := build.trackedStates.LoadOrStore(build.build.ID(), exec.NewRunState())
	return existingState.(exec.RunState)
}

func (build *execBuild) clearRunState() {
	build.trackedStates.Delete(build.build.ID())
}
