package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type TaskEnvValidator struct {
	variablesResolver vars.Variables
	rawTaskEnv        atc.TaskEnv
}

func NewTaskEnvValidator(variables vars.Variables, params atc.TaskEnv) TaskEnvValidator {
	return TaskEnvValidator{
		variablesResolver: variables,
		rawTaskEnv:        params,
	}
}

func (s TaskEnvValidator) Validate() error {
	var params atc.TaskEnv
	return evaluate(s.variablesResolver, s.rawTaskEnv, &params)
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
