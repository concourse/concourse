package creds

import (
	"github.com/concourse/atc"
	"github.com/mitchellh/mapstructure"
)

type Source struct {
	variablesResolver Variables
	rawSource         atc.Source
}

func NewSource(variables Variables, source atc.Source) Source {
	return Source{
		variablesResolver: variables,
		rawSource:         source,
	}
}

func (s Source) Evaluate() (atc.Source, error) {
	var untypedInput interface{}

	err := evaluate(s.variablesResolver, s.rawSource, &untypedInput)
	if err != nil {
		return nil, err
	}

	var metadata mapstructure.Metadata
	var source atc.Source

	msConfig := &mapstructure.DecoderConfig{
		Metadata:         &metadata,
		Result:           &source,
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

	return source, nil
}
