package creds

import (
	"github.com/concourse/concourse/atc"
)

type Params struct {
	variablesResolver Variables
	rawParams         atc.Params
}

func NewParams(variables Variables, params atc.Params) Params {
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
