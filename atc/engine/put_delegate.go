package engine

import (
	"time"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
)

type putDelegate struct {
	exec.BuildStepDelegate

	build       db.Build
	eventOrigin event.Origin
}

func NewPutDelegate(build db.Build, planID atc.PlanID, clock clock.Clock) exec.PutDelegate {
	return &putDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, clock),

		build: build,
		eventOrigin: event.Origin{
			ID: event.OriginID(planID),
		},
	}
}

func (d *putDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializePut{
		Origin: d.eventOrigin,
		Time:   time.Now().Unix(),
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-put-event", err)
		return
	}

	logger.Debug("initializing")
}

func (d *putDelegate) Starting(logger lager.Logger) {
	err := d.build.SaveEvent(event.StartPut{
		Time:   time.Now().Unix(),
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-start-put-event", err)
		return
	}

	logger.Info("starting")
}

func (d *putDelegate) Finished(logger lager.Logger, exitStatus exec.ExitStatus, info exec.VersionInfo) {
	err := d.build.SaveEvent(event.FinishPut{
		Time:            time.Now().Unix(),
		Origin:          d.eventOrigin,
		ExitStatus:      int(exitStatus),
		CreatedVersion:  info.Version,
		CreatedMetadata: info.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-finish-put-event", err)
		return
	}

	logger.Info("finished", lager.Data{
		"exit-status":  exitStatus,
		"version-info": info,
	})
}
