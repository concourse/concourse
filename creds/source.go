package creds

import (
	"encoding/json"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	yaml "gopkg.in/yaml.v2"
)

type Source struct {
	variablesResolver template.Variables
	rawSource         atc.Source
}

func NewSource(variables template.Variables, source atc.Source) *Source {
	return &Source{
		variablesResolver: variables,
		rawSource:         source,
	}
}

func (s *Source) Evaluate() (atc.Source, error) {
	byteSource, err := json.Marshal(s.rawSource)
	if err != nil {
		return nil, err
	}

	tpl := template.NewTemplate(byteSource)
	bytes, err := tpl.Evaluate(s.variablesResolver, nil, template.EvaluateOpts{ExpectAllKeys: true})
	if err != nil {
		return nil, err
	}

	var source atc.Source
	err = yaml.Unmarshal(bytes, &source)
	if err != nil {
		return nil, err
	}

	return source, nil
}
