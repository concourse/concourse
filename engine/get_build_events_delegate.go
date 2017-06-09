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
	implicitOutputsRepo *implicitOutputsRepo
}

func NewGetBuildEventsDelegate(
	build db.Build,
	planID atc.PlanID,
	plan atc.GetPlan,
	implicitOutputsRepo *implicitOutputsRepo,
) exec.BuildEventsDelegate {
	return &getBuildEventsDelegate{
		build:               build,
		eventOrigin:         event.Origin{ID: event.OriginID(planID)},
		plan:                plan,
		implicitOutputsRepo: implicitOutputsRepo,
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

func (d *getBuildEventsDelegate) ActionCompleted(logger lager.Logger, action exec.Action) {
	switch a := action.(type) {
	case *exec.GetAction:
		versionInfo := a.VersionInfo()
		exitStatus := a.ExitStatus()

		if exitStatus == exec.ExitStatus(0) {
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

			d.implicitOutputsRepo.Register(d.plan.Resource, implicitOutput{plan: d.plan, info: versionInfo})
		}

		eventPlan := event.GetPlan{
			Name:     d.plan.Name,
			Resource: d.plan.Resource,
			Type:     d.plan.Type,
		}
		if d.plan.Version != nil {
			// When version is coming from another action, plan does not have a version
			// Also, some resources do not have a version in plan (e.g. archive)
			eventPlan.Version = *d.plan.Version
		}

		err := d.build.SaveEvent(event.FinishGet{
			Origin:          d.eventOrigin,
			Plan:            eventPlan,
			ExitStatus:      int(exitStatus),
			FetchedVersion:  versionInfo.Version,
			FetchedMetadata: versionInfo.Metadata,
		})
		if err != nil {
			logger.Error("failed-to-save-input-event", err)
		}

		logger.Info("finished", lager.Data{"version-info": versionInfo})
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
