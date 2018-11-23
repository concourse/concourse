package creds

import "github.com/concourse/concourse/atc"

type TaskParamsValidator struct {
	variablesResolver Variables
	rawTaskParams     map[string]string
}

func NewTaskParamsValidator(variables Variables, params map[string]string) TaskParamsValidator {
	return TaskParamsValidator{
		variablesResolver: variables,
		rawTaskParams:     params,
	}
}

func (s TaskParamsValidator) Validate() error {
	var params map[string]string
	return evaluate(s.variablesResolver, s.rawTaskParams, &params)
}

type TaskVarsValidator struct {
	variablesResolver Variables
	rawTaskVars       atc.Params
}

func NewTaskVarsValidator(variables Variables, taskVars atc.Params) TaskVarsValidator {
	return TaskVarsValidator{
		variablesResolver: variables,
		rawTaskVars:       taskVars,
	}
}

func (s TaskVarsValidator) Validate() error {
	var params atc.Params
	return evaluate(s.variablesResolver, s.rawTaskVars, &params)
}
