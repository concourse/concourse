package engine

import (
	"sync"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
)

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	DBActionsBuildEventsDelegate(atc.PlanID) exec.ActionsBuildEventsDelegate
	DBTaskBuildEventsDelegate(atc.PlanID) exec.TaskBuildEventsDelegate
	BuildStepDelegate(atc.PlanID) exec.BuildStepDelegate

	Finish(lager.Logger, error, exec.Success, bool)
}

//go:generate counterfeiter . BuildDelegateFactory

type BuildDelegateFactory interface {
	Delegate(db.Build) BuildDelegate
}

type buildDelegateFactory struct{}

func NewBuildDelegateFactory() BuildDelegateFactory {
	return buildDelegateFactory{}
}

func (factory buildDelegateFactory) Delegate(build db.Build) BuildDelegate {
	return newBuildDelegate(build)
}

type delegate struct {
	build               db.Build
	implicitOutputsRepo *implicitOutputsRepo
}

func newBuildDelegate(build db.Build) BuildDelegate {
	return &delegate{
		build: build,

		implicitOutputsRepo: &implicitOutputsRepo{
			outputs: make(map[string]implicitOutput),
			lock:    &sync.Mutex{},
		},
	}
}

func (delegate *delegate) DBActionsBuildEventsDelegate(
	planID atc.PlanID,
) exec.ActionsBuildEventsDelegate {
	return NewDBActionsBuildEventsDelegate(delegate.build, event.Origin{ID: event.OriginID(planID)}, delegate.implicitOutputsRepo)
}

func (delegate *delegate) DBTaskBuildEventsDelegate(
	planID atc.PlanID,
) exec.TaskBuildEventsDelegate {
	return NewDBTaskBuildEventsDelegate(delegate.build, event.Origin{ID: event.OriginID(planID)})
}

func (delegate *delegate) BuildStepDelegate(planID atc.PlanID) exec.BuildStepDelegate {
	return NewBuildStepDelegate(delegate.build, planID, clock.NewClock())
}

func (delegate *delegate) Finish(logger lager.Logger, err error, succeeded exec.Success, aborted bool) {
	if aborted {
		delegate.saveStatus(logger, atc.StatusAborted)

		logger.Info("aborted")
	} else if err != nil {
		delegate.saveStatus(logger, atc.StatusErrored)

		logger.Info("errored", lager.Data{"error": err.Error()})
	} else if bool(succeeded) {
		delegate.saveStatus(logger, atc.StatusSucceeded)

		implicits := logger.Session("implicit-outputs")

		for resourceName, o := range delegate.implicitOutputsRepo.outputs {
			delegate.saveImplicitOutput(implicits.Session(resourceName), resourceName, o.resourceType, o.info)
		}

		logger.Info("succeeded")
	} else {
		delegate.saveStatus(logger, atc.StatusFailed)

		logger.Info("failed")
	}
}

func (delegate *delegate) saveStatus(logger lager.Logger, status atc.BuildStatus) {
	err := delegate.build.Finish(db.BuildStatus(status))
	if err != nil {
		logger.Error("failed-to-finish-build", err)
	}
}

func (delegate *delegate) saveImplicitOutput(logger lager.Logger, resourceName string, resourceType string, info exec.VersionInfo) {
	metadata := make([]db.ResourceMetadataField, len(info.Metadata))
	for i, md := range info.Metadata {
		metadata[i] = db.ResourceMetadataField{
			Name:  md.Name,
			Value: md.Value,
		}
	}

	err := delegate.build.SaveOutput(db.VersionedResource{
		Resource: resourceName,
		Type:     resourceType,
		Version:  db.ResourceVersion(info.Version),
		Metadata: metadata,
	}, false)
	if err != nil {
		logger.Error("failed-to-save", err)
		return
	}

	logger.Info("saved", lager.Data{"resource": resourceName})
}
