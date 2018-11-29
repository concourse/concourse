package template

import (
	"encoding/json"
	"fmt"
	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/hashicorp/go-multierror"
	"regexp"
)

var templateOldStyleFormatRegex = regexp.MustCompile(`\{\{([-\w\p{L}]+)\}\}`)

type TemplateResolver struct {
	configPayload []byte
	params        []boshtemplate.Variables
}

func NewTemplateResolver(configPayload []byte, params []boshtemplate.Variables) TemplateResolver {
	return TemplateResolver{
		configPayload: configPayload,
		params:        params,
	}
}

func (resolver TemplateResolver) Resolve(expectAllKeys bool, allowEmptyInOldStyleTemplates bool) ([]byte, error) {
	var err error

	if PresentDeprecated(resolver.configPayload) {
		resolver.configPayload, err = resolver.ResolveDeprecated(allowEmptyInOldStyleTemplates)
		if err != nil {
			return nil, err
		}
	}

	resolver.configPayload, err = resolver.resolve(expectAllKeys)
	if err != nil {
		return nil, err
	}

	return resolver.configPayload, nil
}

func (resolver TemplateResolver) resolve(expectAllKeys bool) ([]byte, error) {
	tpl := boshtemplate.NewTemplate(resolver.configPayload)

	vars := []boshtemplate.Variables{}
	for i := len(resolver.params) - 1; i >= 0; i-- {
		vars = append(vars, resolver.params[i])
	}

	bytes, err := tpl.Evaluate(boshtemplate.NewMultiVars(vars), nil, boshtemplate.EvaluateOpts{ExpectAllKeys: expectAllKeys})

	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (resolver TemplateResolver) ResolveDeprecated(allowEmpty bool) ([]byte, error) {
	vars := boshtemplate.StaticVariables{}
	for _, variable := range resolver.params {
		// ideally we should deprecate old-style template parameters and remove this all together
		if staticVar, ok := variable.(boshtemplate.StaticVariables); ok {
			for k, v := range staticVar {
				vars[k] = v
			}
		}
	}

	var variableErrors error

	return templateOldStyleFormatRegex.ReplaceAllFunc(resolver.configPayload, func(match []byte) []byte {
		key := string(templateOldStyleFormatRegex.FindSubmatch(match)[1])

		value, found := vars[key]
		if !found && !allowEmpty {
			variableErrors = multierror.Append(variableErrors, fmt.Errorf("unbound variable in template: '%s'", key))
			return match
		}

		saveValue, _ := json.Marshal(value)

		return []byte(saveValue)
	}), variableErrors

}

func PresentDeprecated(content []byte) bool {
	return templateOldStyleFormatRegex.Match(content)
}
