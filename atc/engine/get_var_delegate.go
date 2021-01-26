package engine

import (
	"code.cloudfoundry.org/clock"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/exec"
	"github.com/concourse/concourse/atc/policy"
)

func NewGetVarDelegate(
	build db.Build,
	planID atc.PlanID,
	state exec.RunState,
	clock clock.Clock,
	policyChecker policy.Checker,
	globalSecrets creds.Secrets,
	varSourceConfigs atc.VarSourceConfigs,
) exec.GetDelegate {
	return &getDelegate{
		BuildStepDelegate: NewBuildStepDelegate(build, planID, state, clock, policyChecker, globalSecrets),

		varSourceConfigs: varSourceConfigs,
	}
}

type getVarDelegate struct {
	exec.BuildStepDelegate

	varSourceConfigs atc.VarSourceConfigs
}

func (delegate *getVarDelegate) VarSources() atc.VarSourceConfigs {
	return delegate.varSourceConfigs
}

