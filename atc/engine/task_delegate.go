package engine

import (
	"io"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
	"github.com/concourse/concourse/atc/runtime"
)

func NewTaskDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	dbWorkerFactory db.WorkerFactory,
	lockFactory lock.LockFactory,
) exec.TaskDelegate {
	return &taskDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker),

		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		build:       build,
		clock:       clock,

		dbWorkerFactory: dbWorkerFactory,
		lockFactory:     lockFactory,
	}
}

type taskDelegate struct {
	exec.BuildStepDelegate

	config      atc.TaskConfig
	build       db.Build
	eventOrigin event.Origin
	clock       clock.Clock

	dbWorkerFactory db.WorkerFactory
	lockFactory     lock.LockFactory
}

func (d *taskDelegate) SetTaskConfig(config atc.TaskConfig) {
	d.config = config
}

func (d *taskDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Info("initializing")
}

func (d *taskDelegate) Starting(logger lager.Logger) {
	err := d.build.SaveEvent(event.StartTask{
		Origin:     d.eventOrigin,
		Time:       d.clock.Now().Unix(),
		TaskConfig: event.ShadowTaskConfig(d.config),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
		return
	}

	logger.Debug("starting")
}

func (d *taskDelegate) Finished(
	logger lager.Logger,
	exitStatus exec.ExitStatus,
	chosenWorker runtime.Worker,
) {
	// PR#4398: close to flush stdout and stderr
	d.Stdout().(io.Closer).Close()
	d.Stderr().(io.Closer).Close()

	err := d.build.SaveEvent(event.FinishTask{
		ExitStatus: int(exitStatus),
		Time:       d.clock.Now().Unix(),
		Origin:     d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
		return
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}
