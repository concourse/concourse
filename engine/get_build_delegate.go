package engine

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type getBuildEventsDelegate struct {
	build               db.Build
	eventOrigin         event.Origin
	plan                atc.GetPlan
	implicitOutputsRepo ImplicitOutputsRepo
	resultAction        exec.GetResultAction
}

func NewGetBuildEventsDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.GetPlan,
	implicitOutputsRepo ImplicitOutputsRepo,
	resultAction exec.GetResultAction,
) exec.BuildEventsDelegate {
	return &getBuildEventsDelegate{
		build:               build,
		eventOrigin:         event.Origin{ID: event.OriginID(planID)},
		plan:                plan,
		implicitOutputsRepo: implicitOutputsRepo,
		resultAction:        resultAction,
	}
}

func (d *getBuildEventsDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeGet{
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (d *getBuildEventsDelegate) Failed(logger lager.Logger, errVal error) {
	err := d.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}

	logger.Info("errored", lager.Data{"error": errVal.Error()})
}

func (d *getBuildEventsDelegate) Finished(logger lager.Logger, status exec.ExitStatus) {
	versionInfo, resultPresent := d.resultAction.Result()

	if resultPresent {
		err := d.build.SaveInput(db.BuildInput{
			Name: d.plan.Name,
			VersionedResource: db.VersionedResource{
				Resource: d.plan.Resource,
				Type:     d.plan.Type,
				Version:  db.ResourceVersion(versionInfo.Version),
				Metadata: db.NewResourceMetadataFields(versionInfo.Metadata),
			},
		})
		if err != nil {
			logger.Error("failed-to-save-input", err)
		}
	}

	err := d.build.SaveEvent(event.FinishGet{
		Origin: d.eventOrigin,
		Plan: event.GetPlan{
			Name:     d.plan.Name,
			Resource: d.plan.Resource,
			Type:     d.plan.Type,
			Version:  d.plan.Version,
		},
		ExitStatus:      int(status),
		FetchedVersion:  versionInfo.Version,
		FetchedMetadata: versionInfo.Metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	if resultPresent {
		d.implicitOutputsRepo.Register(d.plan.Resource, implicitOutput{plan: d.plan, info: versionInfo})
	}

	logger.Info("finished", lager.Data{"version-info": versionInfo})
}
