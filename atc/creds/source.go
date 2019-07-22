package creds

import (
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/vars"
)

type Source struct {
	variablesResolver vars.Variables
	rawSource         atc.Source
}

func NewSource(variables vars.Variables, source atc.Source) Source {
	return Source{
		variablesResolver: variables,
		rawSource:         source,
	}
}

func (s Source) Evaluate() (atc.Source, error) {
	var source atc.Source
	err := evaluate(s.variablesResolver, s.rawSource, &source)
	if err != nil {
		return nil, err
	}

	return source, nil
}
