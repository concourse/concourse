package creds

import (
	"encoding/json"

	"github.com/concourse/concourse/vars"
	"github.com/goccy/go-yaml"
)

func evaluate(variablesResolver vars.Variables, in, out any) error {
	byteParams, err := json.Marshal(in)
	if err != nil {
		return err
	}

	tpl := vars.NewTemplate(byteParams)

	bytes, err := tpl.Evaluate(variablesResolver, vars.EvaluateOpts{
		ExpectAllKeys: true,
	})
	if err != nil {
		return err
	}

	return yaml.UnmarshalWithOptions(bytes, out, yaml.UseJSONUnmarshaler())
}
