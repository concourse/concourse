package engine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/tracing"
)

//go:generate counterfeiter . Engine

type Engine interface {
	NewBuild(db.Build) Runnable
	NewCheck(context.Context, db.Check) Runnable
	ReleaseAll(lager.Logger)
}

//go:generate counterfeiter . Runnable

type Runnable interface {
	Run(logger lager.Logger)
}

//go:generate counterfeiter . StepBuilder

type StepBuilder interface {
	BuildStep(lager.Logger, db.Build) (exec.Step, error)
	CheckStep(lager.Logger, db.Check) (exec.Step, error)

	BuildStepErrored(lager.Logger, db.Build, error)
}

func NewEngine(builder StepBuilder) Engine {
	return &engine{
		builder:       builder,
		release:       make(chan bool),
		trackedStates: new(sync.Map),
		waitGroup:     new(sync.WaitGroup),
	}
}

type engine struct {
	builder       StepBuilder
	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup
}

func (engine *engine) ReleaseAll(logger lager.Logger) {
	logger.Info("calling-release-on-builds")

	close(engine.release)

	logger.Info("waiting-on-builds")

	engine.waitGroup.Wait()

	logger.Info("finished-waiting-on-builds")
}

func (engine *engine) NewBuild(build db.Build) Runnable {

	ctx, cancel := context.WithCancel(context.Background())

	return NewBuild(
		ctx,
		cancel,
		build,
		engine.builder,
		engine.release,
		engine.trackedStates,
		engine.waitGroup,
	)
}

func (engine *engine) NewCheck(ctx context.Context, check db.Check) Runnable {

	ctxWithCancel, cancel := context.WithCancel(ctx)

	return NewCheck(
		ctxWithCancel,
		cancel,
		check,
		engine.builder,
		engine.release,
		engine.trackedStates,
		engine.waitGroup,
	)
}

func NewBuild(
	ctx context.Context,
	cancel func(),
	build db.Build,
	builder StepBuilder,
	release chan bool,
	trackedStates *sync.Map,
	waitGroup *sync.WaitGroup,
) Runnable {
	return &engineBuild{
		ctx:    ctx,
		cancel: cancel,

		build:   build,
		builder: builder,

		release:       release,
		trackedStates: trackedStates,
		waitGroup:     waitGroup,
	}
}

type engineBuild struct {
	ctx    context.Context
	cancel func()

	build   db.Build
	builder StepBuilder

	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup

	pipelineCredMgrs []creds.Manager
}

func (b *engineBuild) Run(logger lager.Logger) {
	b.waitGroup.Add(1)
	defer b.waitGroup.Done()

	logger = logger.WithData(lager.Data{
		"build":    b.build.ID(),
		"pipeline": b.build.PipelineName(),
		"job":      b.build.JobName(),
	})

	lock, acquired, err := b.build.AcquireTrackingLock(logger, time.Minute)
	if err != nil {
		logger.Error("failed-to-get-lock", err)
		return
	}

	if !acquired {
		logger.Debug("build-already-tracked")
		return
	}

	defer lock.Release()

	found, err := b.build.Reload()
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-not-found")
		return
	}

	if !b.build.IsRunning() {
		logger.Info("build-already-finished")
		return
	}

	notifier, err := b.build.AbortNotifier()
	if err != nil {
		logger.Error("failed-to-listen-for-aborts", err)
		return
	}

	defer notifier.Close()

	ctx, span := tracing.StartSpan(b.ctx, "build", tracing.Attrs{
		"team":     b.build.TeamName(),
		"pipeline": b.build.PipelineName(),
		"job":      b.build.JobName(),
		"build":    b.build.Name(),
		"build_id": strconv.Itoa(b.build.ID()),
	})
	defer span.End()

	step, err := b.builder.BuildStep(logger, b.build)
	if err != nil {
		logger.Error("failed-to-build-step", err)

		// Fails the build if BuildStep returned error. Because some unrecoverable error,
		// like pipeline var_source is wrong, will cause a build to never start
		// to run.
		b.builder.BuildStepErrored(logger, b.build, err)
		b.finish(logger.Session("finish"), err, false)

		return
	}
	b.trackStarted(logger)
	defer b.trackFinished(logger)

	logger.Info("running")

	state := b.runState()
	defer b.clearRunState()

	noleak := make(chan bool)
	defer close(noleak)

	go func() {
		select {
		case <-noleak:
		case <-notifier.Notify():
			logger.Info("aborting")
			b.cancel()
		}
	}()

	done := make(chan error)
	go func() {
		ctx := lagerctx.NewContext(ctx, logger)
		done <- step.Run(ctx, state)
	}()

	select {
	case <-b.release:
		logger.Info("releasing")

	case err = <-done:
		logger.Debug("engine-build-done")
		if err != nil {
			if _, ok := err.(exec.Retriable); ok {
				return
			}
		}
		b.finish(logger.Session("finish"), err, step.Succeeded())
	}
}

func (b *engineBuild) finish(logger lager.Logger, err error, succeeded bool) {
	if errors.Is(err, context.Canceled) {
		b.saveStatus(logger, atc.StatusAborted)
		logger.Info("aborted")

	} else if err != nil {
		b.saveStatus(logger, atc.StatusErrored)
		logger.Info("errored", lager.Data{"error": err.Error()})

	} else if succeeded {
		b.saveStatus(logger, atc.StatusSucceeded)
		logger.Info("succeeded")

	} else {
		b.saveStatus(logger, atc.StatusFailed)
		logger.Info("failed")
	}
}

func (b *engineBuild) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	if err := b.build.Finish(db.BuildStatus(status)); err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (b *engineBuild) trackStarted(logger lager.Logger) {
	metric.BuildStarted{
		PipelineName: b.build.PipelineName(),
		JobName:      b.build.JobName(),
		BuildName:    b.build.Name(),
		BuildID:      b.build.ID(),
		TeamName:     b.build.TeamName(),
	}.Emit(logger)
}

func (b *engineBuild) trackFinished(logger lager.Logger) {
	found, err := b.build.Reload()
	if err != nil {
		logger.Error("failed-to-load-build-from-db", err)
		return
	}

	if !found {
		logger.Info("build-removed")
		return
	}

	if !b.build.IsRunning() {
		metric.BuildFinished{
			PipelineName:  b.build.PipelineName(),
			JobName:       b.build.JobName(),
			BuildName:     b.build.Name(),
			BuildID:       b.build.ID(),
			BuildStatus:   b.build.Status(),
			BuildDuration: b.build.EndTime().Sub(b.build.StartTime()),
			TeamName:      b.build.TeamName(),
		}.Emit(logger)
	}
}

func (b *engineBuild) runState() exec.RunState {
	id := fmt.Sprintf("build:%v", b.build.ID())
	existingState, _ := b.trackedStates.LoadOrStore(id, exec.NewRunState())
	return existingState.(exec.RunState)
}

func (b *engineBuild) clearRunState() {
	id := fmt.Sprintf("build:%v", b.build.ID())
	b.trackedStates.Delete(id)
}

func NewCheck(
	ctx context.Context,
	cancel func(),
	check db.Check,
	builder StepBuilder,
	release chan bool,
	trackedStates *sync.Map,
	waitGroup *sync.WaitGroup,
) Runnable {
	return &engineCheck{
		ctx:    ctx,
		cancel: cancel,

		check:   check,
		builder: builder,

		release:       release,
		trackedStates: trackedStates,
		waitGroup:     waitGroup,
	}
}

type engineCheck struct {
	ctx    context.Context
	cancel func()

	check   db.Check
	builder StepBuilder

	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup
}

func (c *engineCheck) Run(logger lager.Logger) {
	c.waitGroup.Add(1)
	defer c.waitGroup.Done()

	logger = logger.WithData(lager.Data{
		"check": c.check.ID(),
	})

	lock, acquired, err := c.check.AcquireTrackingLock(logger)
	if err != nil {
		logger.Error("failed-to-get-lock", err)
		return
	}

	if !acquired {
		logger.Debug("check-already-tracked")
		return
	}

	defer lock.Release()

	err = c.check.Start()
	if err != nil {
		logger.Error("failed-to-start-check", err)
		return
	}

	c.trackStarted(logger)
	defer c.trackFinished(logger)

	step, err := c.builder.CheckStep(logger, c.check)
	if err != nil {
		logger.Error("failed-to-create-check-step", err)
		c.check.FinishWithError(fmt.Errorf("create check step: %w", err))
		return
	}

	logger.Info("running")

	state := c.runState()
	defer c.clearRunState()

	done := make(chan error)
	go func() {
		ctx := lagerctx.NewContext(c.ctx, logger)
		done <- step.Run(ctx, state)
	}()

	select {
	case <-c.release:
		logger.Info("releasing")

	case err = <-done:
		if err != nil {
			logger.Info("errored", lager.Data{"error": err.Error()})
			c.check.FinishWithError(fmt.Errorf("run check step: %w", err))
		} else {
			logger.Info("succeeded")
			if err = c.check.Finish(); err != nil {
				logger.Error("failed-to-finish-check", err)
			}
		}
	}
}

func (c *engineCheck) runState() exec.RunState {
	id := fmt.Sprintf("check:%v", c.check.ID())
	existingState, _ := c.trackedStates.LoadOrStore(id, exec.NewRunState())
	return existingState.(exec.RunState)
}

func (c *engineCheck) clearRunState() {
	id := fmt.Sprintf("check:%v", c.check.ID())
	c.trackedStates.Delete(id)
}

func (c *engineCheck) trackStarted(logger lager.Logger) {
	metric.ChecksStarted.Inc()
}

func (c *engineCheck) trackFinished(logger lager.Logger) {
	switch c.check.Status() {
	case db.CheckStatusErrored:
		metric.ChecksFinishedWithError.Inc()
	case db.CheckStatusSucceeded:
		metric.ChecksFinishedWithSuccess.Inc()
	default:
		logger.Info("unexpected-check-status", lager.Data{"status": c.check.Status()})
	}
}
