package engine

import (
	"context"

	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/exec"
)

//go:generate counterfeiter . BuildDelegate

type BuildDelegate interface {
	GetDelegate(atc.PlanID) exec.GetDelegate
	PutDelegate(atc.PlanID) exec.PutDelegate
	TaskDelegate(atc.PlanID) exec.TaskDelegate

	BuildStepDelegate(atc.PlanID) exec.BuildStepDelegate

	Finish(lager.Logger, error, bool)
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
	build db.Build
}

func newBuildDelegate(build db.Build) BuildDelegate {
	return &delegate{
		build: build,
	}
}

func (delegate *delegate) GetDelegate(planID atc.PlanID) exec.GetDelegate {
	return NewGetDelegate(delegate.build, planID, clock.NewClock())
}

func (delegate *delegate) PutDelegate(planID atc.PlanID) exec.PutDelegate {
	return NewPutDelegate(delegate.build, planID, clock.NewClock())
}

func (delegate *delegate) TaskDelegate(planID atc.PlanID) exec.TaskDelegate {
	return NewTaskDelegate(delegate.build, planID, clock.NewClock())
}

func (delegate *delegate) BuildStepDelegate(planID atc.PlanID) exec.BuildStepDelegate {
	return NewBuildStepDelegate(delegate.build, planID, clock.NewClock())
}

func (delegate *delegate) Finish(logger lager.Logger, err error, succeeded bool) {
	if err == context.Canceled {
		delegate.saveStatus(logger, atc.StatusAborted)
		logger.Info("aborted")
	} else if err != nil {
		delegate.saveStatus(logger, atc.StatusErrored)
		logger.Info("errored", lager.Data{"error": err.Error()})
	} else if succeeded {
		delegate.saveStatus(logger, atc.StatusSucceeded)
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
