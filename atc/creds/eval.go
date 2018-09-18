package creds

import (
	"encoding/json"

	"github.com/cloudfoundry/bosh-cli/director/template"
	yaml "gopkg.in/yaml.v2"
)

func evaluate(variablesResolver Variables, in, out interface{}) error {
	byteParams, err := json.Marshal(in)
	if err != nil {
		return err
	}

	tpl := template.NewTemplate(byteParams)

	bytes, err := tpl.Evaluate(variablesResolver, nil, template.EvaluateOpts{
		ExpectAllKeys: true,
	})
	if err != nil {
		return err
	}

	return yaml.Unmarshal(bytes, out)
}
