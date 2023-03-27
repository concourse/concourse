package engine

import (
	"code.cloudfoundry.org/clock"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
)

func NewSetPipelineStepDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
) *setPipelineStepDelegate {
	return &setPipelineStepDelegate{
		buildStepDelegate{
			build:         build,
			planID:        planID,
			clock:         clock,
			state:         state,
			stdout:        nil,
			stderr:        nil,
			policyChecker: policyChecker,
		},
	}
}

type setPipelineStepDelegate struct {
	buildStepDelegate
}

func (delegate *setPipelineStepDelegate) SetPipelineChanged(logger lager.Logger, changed bool) {
	err := delegate.build.SaveEvent(event.SetPipelineChanged{
		Origin: event.Origin{
			ID: event.OriginID(delegate.planID),
		},
		Changed: changed,
	})
	if err != nil {
		logger.Error("failed-to-save-set-pipeline-changed-event", err)
		return
	}

	logger.Debug("set pipeline changed")
}

func (delegate *setPipelineStepDelegate) CheckRunSetPipelinePolicy(atcConfig *atc.Config) error {
	if !delegate.policyChecker.ShouldCheckAction(policy.ActionRunSetPipeline) {
		return nil
	}

	return delegate.checkPolicy(policy.PolicyCheckInput{
		Action:   policy.ActionRunSetPipeline,
		Team:     delegate.build.TeamName(),
		Pipeline: delegate.build.PipelineName(),
		Data:     atcConfig,
	})
}
