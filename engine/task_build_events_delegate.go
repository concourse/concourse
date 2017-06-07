package engine

import (
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type taskBuildEventsDelegate struct {
	build            db.Build
	eventOrigin      event.Origin
	plan             atc.TaskPlan
	taskResultAction exec.TaskResultAction
}

func NewTaskBuildEventsDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.TaskPlan,
	taskResultAction exec.TaskResultAction,
) exec.BuildEventsDelegate {
	return &taskBuildEventsDelegate{
		build:            build,
		eventOrigin:      event.Origin{ID: event.OriginID(planID)},
		plan:             plan,
		taskResultAction: taskResultAction,
	}
}

func (d *taskBuildEventsDelegate) Initializing(logger lager.Logger) {
	// TODO: add task config
	err := d.build.SaveEvent(event.InitializeTask{
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (d *taskBuildEventsDelegate) Failed(logger lager.Logger, errVal error) {
	err := d.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}

	logger.Info("errored", lager.Data{"error": errVal.Error()})
}

func (d *taskBuildEventsDelegate) Finished(logger lager.Logger) {
	exitStatus := d.taskResultAction.ExitStatus()
	err := d.build.SaveEvent(event.FinishTask{
		ExitStatus: int(exitStatus),
		Time:       time.Now().Unix(),
		Origin:     d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-event", err)
	}

	logger.Info("finished", lager.Data{"exit-status": exitStatus})
}
