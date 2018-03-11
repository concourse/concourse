package engine

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type dbActionsBuildEventsDelegate struct {
	build       db.Build
	eventOrigin event.Origin
}

func NewDBActionsBuildEventsDelegate(
	build db.Build,
	eventOrigin event.Origin,
) exec.ActionsBuildEventsDelegate {
	return &dbActionsBuildEventsDelegate{
		build:       build,
		eventOrigin: eventOrigin,
	}
}

func (d *dbActionsBuildEventsDelegate) ActionCompleted(logger lager.Logger, action exec.Action) {
	switch a := action.(type) {
	case *exec.PutAction:
		versionInfo := a.VersionInfo()
		exitStatus := a.ExitStatus()

		err := d.build.SaveEvent(event.FinishPut{
			Origin: d.eventOrigin,
			Plan: event.PutPlan{
				Name:     a.Name,
				Resource: a.Resource,
				Type:     a.Type,
			},
			ExitStatus:      int(exitStatus),
			CreatedVersion:  versionInfo.Version,
			CreatedMetadata: versionInfo.Metadata,
		})
		if err != nil {
			logger.Error("failed-to-save-input-event", err)
			return
		}

		if exitStatus == exec.ExitStatus(0) {
			err := d.build.SaveOutput(
				db.VersionedResource{
					Resource: a.Resource,
					Type:     a.Type,
					Version:  db.ResourceVersion(versionInfo.Version),
					Metadata: db.NewResourceMetadataFields(versionInfo.Metadata),
				},
				true,
			)
			if err != nil {
				logger.Error("failed-to-save-output", err)
				return
			}
		}

		logger.Info("finished", lager.Data{"version-info": versionInfo})
	default:
		return
	}
}

func (d *dbActionsBuildEventsDelegate) Failed(logger lager.Logger, errVal error) {
	err := d.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}

	logger.Info("errored", lager.Data{"error": errVal.Error()})
}
