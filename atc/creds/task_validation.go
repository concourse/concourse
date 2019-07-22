package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type TaskParamsValidator struct {
	variablesResolver vars.Variables
	rawTaskParams     atc.Params
}

func NewTaskParamsValidator(variables vars.Variables, params atc.Params) TaskParamsValidator {
	return TaskParamsValidator{
		variablesResolver: variables,
		rawTaskParams:     params,
	}
}

func (s TaskParamsValidator) Validate() error {
	var params atc.TaskEnv
	return evaluate(s.variablesResolver, s.rawTaskParams, &params)
}

type TaskVarsValidator struct {
	variablesResolver vars.Variables
	rawTaskVars       atc.Params
}

func NewTaskVarsValidator(variables vars.Variables, taskVars atc.Params) TaskVarsValidator {
	return TaskVarsValidator{
		variablesResolver: variables,
		rawTaskVars:       taskVars,
	}
}

func (s TaskVarsValidator) Validate() error {
	var params atc.Params
	return evaluate(s.variablesResolver, s.rawTaskVars, &params)
}
