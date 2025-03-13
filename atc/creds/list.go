package creds

import "github.com/concourse/concourse/vars"

type List struct {
	variablesResolver vars.Variables
	raw               any
}

func NewList(variables vars.Variables, raw any) List {
	return List{
		variablesResolver: variables,
		raw:               raw,
	}
}

func (l List) Evaluate() ([]any, error) {
	var list []any

	err := evaluate(l.variablesResolver, l.raw, &list)
	if err != nil {
		return nil, err
	}

	return list, nil
}
