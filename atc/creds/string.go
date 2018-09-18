package creds

import (
	"github.com/concourse/atc"
	"github.com/mitchellh/mapstructure"
)

type String struct {
	variablesResolver Variables
	rawCredString     string
}

func NewString(variables Variables, credString string) String {
	return String{
		variablesResolver: variables,
		rawCredString:     credString,
	}
}

func (s String) Evaluate() (string, error) {
	var untypedInput interface{}

	err := evaluate(s.variablesResolver, s.rawCredString, &untypedInput)
	if err != nil {
		return s.rawCredString, err
	}

	var metadata mapstructure.Metadata
	var credString string

	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &metadata,
		Result:           &credString,
		WeaklyTypedInput: true,
		DecodeHook:       atc.SanitizeDecodeHook,
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		return s.rawCredString, err
	}

	if err := decoder.Decode(untypedInput); err != nil {
		return s.rawCredString, err
	}

	return credString, nil
}
