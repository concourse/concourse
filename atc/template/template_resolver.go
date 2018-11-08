package template

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director/template"
	temp "github.com/concourse/concourse/fly/template"
	"gopkg.in/yaml.v2"
)

type TemplateResolver struct {
}

func (templateResolver TemplateResolver) Resolve(configPayload []byte, paramPayloads [][]byte, flagVars template.StaticVariables, allowEmptyInOldStyleTemplates bool) ([]byte, error) {
	var err error
	if temp.Present(configPayload) {
		configPayload, err = templateResolver.resolveDeprecatedTemplateStyle(configPayload, paramPayloads, flagVars, allowEmptyInOldStyleTemplates)
		if err != nil {
			return nil, fmt.Errorf("could not resolve old-style template vars: %s", err.Error())
		}
	}

	configPayload, err = templateResolver.resolveTemplates(configPayload, paramPayloads, flagVars)
	if err != nil {
		return nil, fmt.Errorf("could not resolve template vars: %s", err.Error())
	}

	return configPayload, nil
}

func (templateResolver TemplateResolver) resolveTemplates(configPayload []byte, paramPayloads [][]byte, flagVars template.StaticVariables) ([]byte, error) {
	tpl := template.NewTemplate(configPayload)

	vars := []template.Variables{}
	if flagVars != nil {
		vars = append(vars, flagVars)
	}
	for i := len(paramPayloads) - 1; i >= 0; i-- {
		payload := paramPayloads[i]

		var staticVars template.StaticVariables
		err := yaml.Unmarshal(payload, &staticVars)
		if err != nil {
			return nil, err
		}

		vars = append(vars, staticVars)
	}

	bytes, err := tpl.Evaluate(template.NewMultiVars(vars), nil, template.EvaluateOpts{})
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (templateResolver TemplateResolver) resolveDeprecatedTemplateStyle(
	configPayload []byte,
	paramPayloads [][]byte,
	flagVars template.StaticVariables,
	allowEmpty bool,
) ([]byte, error) {
	vars := temp.Variables{}
	for _, payload := range paramPayloads {
		var payloadVars temp.Variables
		err := yaml.Unmarshal(payload, &payloadVars)
		if err != nil {
			return nil, err
		}

		vars = vars.Merge(payloadVars)
	}

	for k, v := range flagVars {
		vars[k] = v.(string)
	}

	return temp.Evaluate(configPayload, vars, allowEmpty)
}
