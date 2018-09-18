package creds

import (
	"github.com/concourse/atc"
	"github.com/mitchellh/mapstructure"
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
	var untypedInput interface{}

	err := evaluate(p.variablesResolver, p.rawParams, &untypedInput)
	if err != nil {
		return nil, err
	}

	var metadata mapstructure.Metadata
	var params atc.Params

	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &metadata,
		Result:           &params,
		WeaklyTypedInput: true,
		DecodeHook:       atc.SanitizeDecodeHook,
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(untypedInput); err != nil {
		return nil, err
	}

	return params, nil
}
