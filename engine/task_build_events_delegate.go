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
	build       db.Build
	eventOrigin event.Origin
	plan        atc.TaskPlan
}

func NewTaskBuildEventsDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.TaskPlan,
) exec.BuildEventsDelegate {
	return &taskBuildEventsDelegate{
		build:       build,
		eventOrigin: event.Origin{ID: event.OriginID(planID)},
		plan:        plan,
	}
}

func (d *taskBuildEventsDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeTask{
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (d *taskBuildEventsDelegate) ActionCompleted(logger lager.Logger, action exec.Action) {
	switch a := action.(type) {
	case *exec.FetchConfigAction:
		taskConfig := a.Result()
		err := d.build.SaveEvent(event.StartTask{
			Origin:     d.eventOrigin,
			TaskConfig: event.ShadowTaskConfig(taskConfig),
			Time:       time.Now().Unix(),
		})
		if err != nil {
			logger.Error("failed-to-save-start-task-event", err)
			return
		}
	case *exec.TaskAction:
		exitStatus := a.ExitStatus()
		err := d.build.SaveEvent(event.FinishTask{
			ExitStatus: int(exitStatus),
			Time:       time.Now().Unix(),
			Origin:     d.eventOrigin,
		})
		if err != nil {
			logger.Error("failed-to-save-finish-event", err)
		}

		logger.Info("finished", lager.Data{"exit-status": exitStatus})
	default:
		return
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
