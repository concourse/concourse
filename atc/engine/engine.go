package engine

import (
	"context"
	"errors"
	"sync"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/metric"
	"github.com/concourse/concourse/atc/util"
	"github.com/concourse/concourse/tracing"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . Engine
type Engine interface {
	NewBuild(db.Build) Runnable

	Drain(context.Context)
}

//counterfeiter:generate . Runnable
type Runnable interface {
	Run(context.Context)
}

func NewEngine(
	stepperFactory StepperFactory,
	secrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
) Engine {
	return &engine{
		stepperFactory: stepperFactory,
		release:        make(chan bool),
		trackedStates:  new(sync.Map),
		waitGroup:      new(sync.WaitGroup),

		globalSecrets: secrets,
		varSourcePool: varSourcePool,
	}
}

type engine struct {
	stepperFactory StepperFactory
	release        chan bool
	trackedStates  *sync.Map
	waitGroup      *sync.WaitGroup

	globalSecrets creds.Secrets
	varSourcePool creds.VarSourcePool
}

func (engine *engine) Drain(ctx context.Context) {
	logger := lagerctx.FromContext(ctx)

	logger.Info("start")
	defer logger.Info("done")

	close(engine.release)

	logger.Info("waiting")

	engine.waitGroup.Wait()
}

func (engine *engine) NewBuild(build db.Build) Runnable {
	return NewBuild(
		build,
		engine.stepperFactory,
		engine.globalSecrets,
		engine.varSourcePool,
		engine.release,
		engine.trackedStates,
		engine.waitGroup,
	)
}

func NewBuild(
	build db.Build,
	builder StepperFactory,
	globalSecrets creds.Secrets,
	varSourcePool creds.VarSourcePool,
	release chan bool,
	trackedStates *sync.Map,
	waitGroup *sync.WaitGroup,
) Runnable {
	return &engineBuild{
		build:   build,
		builder: builder,

		globalSecrets: globalSecrets,
		varSourcePool: varSourcePool,

		release:       release,
		trackedStates: trackedStates,
		waitGroup:     waitGroup,
	}
}

type engineBuild struct {
	build   db.Build
	builder StepperFactory

	globalSecrets creds.Secrets
	varSourcePool creds.VarSourcePool

	release       chan bool
	trackedStates *sync.Map
	waitGroup     *sync.WaitGroup
}

func (b *engineBuild) Run(ctx context.Context) {
	b.waitGroup.Add(1)
	defer b.waitGroup.Done()

	logger := lagerctx.FromContext(ctx).WithData(b.build.LagerData())

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

	ctx, span := tracing.StartSpanFollowing(ctx, b.build, "build", b.build.TracingAttrs())
	defer span.End()

	stepper, err := b.builder.StepperForBuild(b.build)
	if err != nil {
		logger.Error("failed-to-construct-build-stepper", err)

		// Fails the build if BuildStep returned an error because such unrecoverable
		// errors will cause a build to never start to run.
		b.buildStepErrored(logger, err.Error())
		b.finish(logger.Session("finish"), err, false)

		return
	}

	b.trackStarted(logger)
	defer b.trackFinished(logger)

	logger.Info("running")

	state, err := b.runState(logger, stepper)
	if err != nil {
		logger.Error("failed-to-create-run-state", err)

		// Fails the build if fetching the pipeline variables fails, as these errors
		// are unrecoverable - e.g. if pipeline var_sources is wrong
		b.buildStepErrored(logger, err.Error())
		b.finish(logger.Session("finish"), err, false)

		return
	}
	defer b.clearRunState()

	ctx, cancel := context.WithCancel(ctx)

	noleak := make(chan bool)
	defer close(noleak)

	go func() {
		select {
		case <-noleak:
		case <-notifier.Notify():
			logger.Info("aborting")
			cancel()
		}
	}()

	var succeeded bool
	var runErr error

	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() {
			err := util.DumpPanic(recover(), "running build plan %d", b.build.ID())
			if err != nil {
				logger.Error("panic-in-engine-build-step-run", err)
				runErr = err
			}
		}()

		succeeded, runErr = state.Run(lagerctx.NewContext(ctx, logger), b.build.PrivatePlan())
	}()

	select {
	case <-b.release:
		logger.Info("releasing")

	case <-done:
		if errors.As(runErr, &exec.Retriable{}) {
			return
		}

		// An in-memory build only generates a real build id once start to run,
		// so let's update logger with the latest lager data.
		b.finish(logger.Session("finish").WithData(b.build.LagerData()), runErr, succeeded)
	}
}

func (b *engineBuild) buildStepErrored(logger lager.Logger, message string) {
	err := b.build.SaveEvent(event.Error{
		Message: message,
		Origin: event.Origin{
			ID: event.OriginID(b.build.PrivatePlan().ID),
		},
		Time: time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
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
	if b.build.Name() != db.CheckBuildName {
		metric.BuildStarted{
			Build: b.build,
		}.Emit(logger)
	} else {
		metric.CheckBuildStarted{
			Build: b.build,
		}.Emit(logger)
	}
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
		if b.build.Name() != db.CheckBuildName {
			metric.BuildFinished{
				Build: b.build,
			}.Emit(logger)
		} else {
			metric.CheckBuildFinished{
				Build: b.build,
			}.Emit(logger)
		}
	}
}

func (b *engineBuild) runState(logger lager.Logger, stepper exec.Stepper) (exec.RunState, error) {
	id := b.build.RunStateID()
	existingState, ok := b.trackedStates.Load(id)
	if ok {
		return existingState.(exec.RunState), nil
	}
	credVars, err := b.build.Variables(logger, b.globalSecrets, b.varSourcePool)
	if err != nil {
		return nil, err
	}
	state, _ := b.trackedStates.LoadOrStore(id, exec.NewRunState(stepper, credVars, atc.EnableRedactSecrets))
	return state.(exec.RunState), nil
}

func (b *engineBuild) clearRunState() {
	b.trackedStates.Delete(b.build.RunStateID())
}
