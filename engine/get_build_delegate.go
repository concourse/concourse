package engine

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

type getBuildDelegate struct {
	build               db.Build
	eventOrigin         event.Origin
	plan                atc.GetPlan
	implicitOutputsRepo ImplicitOutputsRepo
	result              *atc.VersionInfo
}

func NewGetBuildDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.GetPlan,
	implicitOutputsRepo ImplicitOutputsRepo,
	result *atc.VersionInfo,
) exec.BuildDelegate {
	return &getBuildDelegate{
		build:               build,
		eventOrigin:         event.Origin{ID: event.OriginID(planID)},
		plan:                plan,
		implicitOutputsRepo: implicitOutputsRepo,
		result:              result,
	}
}

func (d *getBuildDelegate) Initializing(logger lager.Logger) {
	err := d.build.SaveEvent(event.InitializeGet{
		Origin: d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-initialize-event", err)
	}
}

func (d *getBuildDelegate) Failed(logger lager.Logger, errVal error) {
	err := d.build.SaveEvent(event.Error{
		Message: errVal.Error(),
		Origin:  d.eventOrigin,
	})
	if err != nil {
		logger.Error("failed-to-save-error-event", err)
	}

	logger.Info("errored", lager.Data{"error": errVal.Error()})
}

func (d *getBuildDelegate) Finished(logger lager.Logger, status exec.ExitStatus) {
	var version atc.Version
	var metadata []atc.MetadataField

	if d.result != nil {
		err := d.build.SaveInput(db.BuildInput{
			Name: d.plan.Name,
			VersionedResource: db.VersionedResource{
				Resource: d.plan.Resource,
				Type:     d.plan.Type,
				Version:  db.ResourceVersion(d.result.Version),
				Metadata: db.NewResourceMetadataFields(d.result.Metadata),
			},
		})
		if err != nil {
			logger.Error("failed-to-save-input", err)
		}

		version = d.result.Version
		metadata = d.result.Metadata
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
		FetchedVersion:  version,
		FetchedMetadata: metadata,
	})
	if err != nil {
		logger.Error("failed-to-save-input-event", err)
	}

	if d.result != nil {
		d.implicitOutputsRepo.Register(d.plan.Resource, implicitOutput{plan: d.plan, info: *d.result})
	}

	logger.Info("finished", lager.Data{"version-info": d.result})
}
