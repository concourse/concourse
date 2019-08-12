package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type Params struct {
	variablesResolver vars.Variables
	rawParams         atc.Params
}

func NewParams(variables vars.Variables, params atc.Params) Params {
	return Params{
		variablesResolver: variables,
		rawParams:         params,
	}
}

func (p Params) Evaluate() (atc.Params, error) {
	var params atc.Params
	err := evaluate(p.variablesResolver, p.rawParams, &params)
	if err != nil {
		return nil, err
	}

	return params, nil
}
