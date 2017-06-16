package engine

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type dbTaskBuildEventsDelegate struct {
	build       db.Build
	eventOrigin event.Origin
}

func NewDBTaskBuildEventsDelegate(
	build db.Build,
	eventOrigin event.Origin,
) exec.TaskBuildEventsDelegate {
	return &dbTaskBuildEventsDelegate{
		build:       build,
		eventOrigin: eventOrigin,
	}
}

func (d *dbTaskBuildEventsDelegate) Initializing(logger lager.Logger, taskConfig atc.TaskConfig) {
	err := d.build.SaveEvent(event.InitializeTask{
		Origin:     d.eventOrigin,
		TaskConfig: event.ShadowTaskConfig(taskConfig),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
	}
}

func (d *dbTaskBuildEventsDelegate) Starting(logger lager.Logger, taskConfig atc.TaskConfig) {
	err := d.build.SaveEvent(event.StartTask{
		Origin:     d.eventOrigin,
		TaskConfig: event.ShadowTaskConfig(taskConfig),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-task-event", err)
	}
}
