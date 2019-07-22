package creds

import (
	"encoding/json"

	"github.com/concourse/concourse/atc/template"
	"github.com/ghodss/yaml"
)

func evaluate(variablesResolver Variables, in, out interface{}) error {
	byteParams, err := json.Marshal(in)
	if err != nil {
		return err
	}

	tpl := template.NewTemplate(byteParams)

	bytes, err := tpl.Evaluate(variablesResolver, template.EvaluateOpts{
		ExpectAllKeys: true,
	})
	if err != nil {
		return err
	}

	return yaml.Unmarshal(bytes, out)
}
