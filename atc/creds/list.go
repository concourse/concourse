package creds

import "github.com/concourse/concourse/vars"

type List struct {
	variablesResolver vars.Variables
	raw               interface{}
}

func NewList(variables vars.Variables, raw interface{}) List {
	return List{
		variablesResolver: variables,
		raw:               raw,
	}
}

func (l List) Evaluate() ([]interface{}, error) {
	var list []interface{}

	err := evaluate(l.variablesResolver, l.raw, &list)
	if err != nil {
		return nil, err
	}

	return list, nil
}
