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
	params        []boshtemplate.StaticVariables
}

func NewTemplateResolver(configPayload []byte, params []boshtemplate.StaticVariables) TemplateResolver {
	return TemplateResolver{
		configPayload: configPayload,
		params:        params,
	}
}

func (resolver TemplateResolver) Resolve(allowEmptyInOldStyleTemplates bool) ([]byte, error) {
	var err error

	if PresentDeprecated(resolver.configPayload) {
		resolver.configPayload, err = resolver.ResolveDeprecated(allowEmptyInOldStyleTemplates)
		if err != nil {
			return nil, fmt.Errorf("could not resolve old-style template vars: %s", err.Error())
		}
	}

	resolver.configPayload, err = resolver.resolve()
	if err != nil {
		return nil, fmt.Errorf("could not resolve template vars: %s", err.Error())
	}

	return resolver.configPayload, nil
}

func (resolver TemplateResolver) resolve() ([]byte, error) {
	tpl := boshtemplate.NewTemplate(resolver.configPayload)

	vars := []boshtemplate.Variables{}
	for i := len(resolver.params) - 1; i >= 0; i-- {
		vars = append(vars, resolver.params[i])
	}

	bytes, err := tpl.Evaluate(boshtemplate.NewMultiVars(vars), nil, boshtemplate.EvaluateOpts{})

	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func (resolver TemplateResolver) ResolveDeprecated(allowEmpty bool) ([]byte, error) {
	vars := boshtemplate.StaticVariables{}
	for _, payload := range resolver.params {
		for k, v := range payload {
			vars[k] = v
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
