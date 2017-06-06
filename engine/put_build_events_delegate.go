package engine

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type putBuildEventsDelegate struct {
	build               db.Build
	eventOrigin         event.Origin
	plan                atc.PutPlan
	implicitOutputsRepo ImplicitOutputsRepo
	resultAction        exec.PutResultAction
}

func NewPutBuildEventsDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.PutPlan,
	implicitOutputsRepo ImplicitOutputsRepo,
	resultAction exec.PutResultAction,
) exec.BuildEventsDelegate {
	return &putBuildEventsDelegate{
		build:               build,
		eventOrigin:         event.Origin{ID: event.OriginID(planID)},
		plan:                plan,
		implicitOutputsRepo: implicitOutputsRepo,
		resultAction:        resultAction,
	}
}

func (d *putBuildEventsDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializePut{
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (d *putBuildEventsDelegate) Failed(logger lager.Logger, errVal error) {
	err := d.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}

	logger.Info("errored", lager.Data{"error": errVal.Error()})
}

func (d *putBuildEventsDelegate) Finished(logger lager.Logger, status exec.ExitStatus) {
	d.implicitOutputsRepo.Unregister(d.plan.Resource)

	versionInfo, resultPresent := d.resultAction.Result()
	err := d.build.SaveEvent(event.FinishPut{
		Origin: d.eventOrigin,
		Plan: event.PutPlan{
			Name:     d.plan.Name,
			Resource: d.plan.Resource,
			Type:     d.plan.Type,
		},
		ExitStatus:      int(status),
		CreatedVersion:  versionInfo.Version,
		CreatedMetadata: versionInfo.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	if resultPresent {
		err := d.build.SaveOutput(
			db.VersionedResource{
				Resource: d.plan.Resource,
				Type:     d.plan.Type,
				Version:  db.ResourceVersion(versionInfo.Version),
				Metadata: db.NewResourceMetadataFields(versionInfo.Metadata),
			},
			true,
		)
		if err != nil {
			logger.Error("failed-to-save-output", err)
		}
	}

	logger.Info("finished", lager.Data{"version-info": versionInfo})
}
