package creds

import "github.com/cloudfoundry/bosh-cli/director/template"

type TaskParams struct {
	variablesResolver template.Variables
	rawTaskParams     map[string]string
}

func NewTaskParams(variables template.Variables, params map[string]string) *TaskParams {
	return &TaskParams{
		variablesResolver: variables,
		rawTaskParams:     params,
	}
}

func (s *TaskParams) Evaluate() (map[string]string, error) {
	var params map[string]string
	err := evaluate(s.variablesResolver, s.rawTaskParams, &params)
	if err != nil {
		return nil, err
	}

	return params, nil
}
