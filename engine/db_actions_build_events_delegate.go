package engine

import (
	"time"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type dbActionsBuildEventsDelegate struct {
	build               db.Build
	eventOrigin         event.Origin
	implicitOutputsRepo *implicitOutputsRepo
}

func NewDBActionsBuildEventsDelegate(
	build db.Build,
	eventOrigin event.Origin,
	implicitOutputsRepo *implicitOutputsRepo,
) exec.ActionsBuildEventsDelegate {
	return &dbActionsBuildEventsDelegate{
		build:               build,
		eventOrigin:         eventOrigin,
		implicitOutputsRepo: implicitOutputsRepo,
	}
}

func (d *dbActionsBuildEventsDelegate) ActionCompleted(logger lager.Logger, action exec.Action) {
	switch a := action.(type) {
	case *exec.TaskAction:
		exitStatus := a.ExitStatus()
		err := d.build.SaveEvent(event.FinishTask{
			ExitStatus: int(exitStatus),
			Time:       time.Now().Unix(),
			Origin:     d.eventOrigin,
		})
		if err != nil {
			logger.Error("failed-to-save-finish-event", err)
			return
		}

		logger.Info("finished", lager.Data{"exit-status": exitStatus})
	case *exec.GetAction:
		versionInfo := a.VersionInfo()
		exitStatus := a.ExitStatus()

		if exitStatus == exec.ExitStatus(0) {
			err := d.build.SaveInput(db.BuildInput{
				Name: a.Name,
				VersionedResource: db.VersionedResource{
					Resource: a.Resource,
					Type:     a.Type,
					Version:  db.ResourceVersion(versionInfo.Version),
					Metadata: db.NewResourceMetadataFields(versionInfo.Metadata),
				},
			})
			if err != nil {
				logger.Error("failed-to-save-input", err)
			}

			d.implicitOutputsRepo.Register(a.Resource, implicitOutput{
				resourceType: a.Type,
				info:         versionInfo,
			})
		}

		eventPlan := event.GetPlan{
			Name:     a.Name,
			Resource: a.Resource,
			Type:     a.Type,
		}
		version, err := a.VersionSource.GetVersion()
		if err != nil {
			logger.Error("failed-to-get-version-from-get-action-version-source", err)
			return
		}

		eventPlan.Version = version

		err = d.build.SaveEvent(event.FinishGet{
			Origin:          d.eventOrigin,
			Plan:            eventPlan,
			ExitStatus:      int(exitStatus),
			FetchedVersion:  versionInfo.Version,
			FetchedMetadata: versionInfo.Metadata,
		})
		if err != nil {
			logger.Error("failed-to-save-input-event", err)
			return
		}

		logger.Info("finished", lager.Data{"version-info": versionInfo})
	case *exec.PutAction:
		d.implicitOutputsRepo.Unregister(a.Resource)

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
