package creds

type TaskParams struct {
	variablesResolver Variables
	rawTaskParams     map[string]string
}

func NewTaskParams(variables Variables, params map[string]string) TaskParams {
	return TaskParams{
		variablesResolver: variables,
		rawTaskParams:     params,
	}
}

func (s TaskParams) Evaluate() (map[string]string, error) {
	var params map[string]string
	err := evaluate(s.variablesResolver, s.rawTaskParams, &params)
	if err != nil {
		return nil, err
	}

	return params, nil
}
