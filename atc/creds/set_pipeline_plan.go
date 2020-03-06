package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type SetPipelinePlan struct {
	variablesResolver vars.Variables
	rawPlan           atc.SetPipelinePlan
}

func NewSetPipelinePlan(variables vars.Variables, plan atc.SetPipelinePlan) SetPipelinePlan {
	return SetPipelinePlan{
		variablesResolver: variables,
		rawPlan:           plan,
	}
}

func (s SetPipelinePlan) Evaluate() (atc.SetPipelinePlan, error) {
	var plan atc.SetPipelinePlan
	err := evaluate(s.variablesResolver, s.rawPlan, &plan)
	if err != nil {
		return atc.SetPipelinePlan{}, err
	}

	return plan, nil
}
