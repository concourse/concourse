package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type LoadVarPlan struct {
	variablesResolver vars.Variables
	rawPlan           atc.LoadVarPlan
}

func NewLoadVarPlan(variables vars.Variables, plan atc.LoadVarPlan) LoadVarPlan {
	return LoadVarPlan{
		variablesResolver: variables,
		rawPlan:           plan,
	}
}

func (s LoadVarPlan) Evaluate() (atc.LoadVarPlan, error) {
	var plan atc.LoadVarPlan

	// Name of load_var should not be interpolated.
	name := s.rawPlan.Name

	err := evaluate(s.variablesResolver, s.rawPlan, &plan)
	if err != nil {
		return atc.LoadVarPlan{}, err
	}
	plan.Name = name

	return plan, nil
}
