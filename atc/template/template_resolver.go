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

// Creates a template resolver, given a configPayload and a slice of param sources. If more than
// one param source is specified, they will be tried for variable lookup in the provided order.
// See implementation of boshtemplate.NewMultiVars for details.
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
	bytes, err := tpl.Evaluate(boshtemplate.NewMultiVars(resolver.params), nil, boshtemplate.EvaluateOpts{ExpectAllKeys: expectAllKeys})
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (resolver TemplateResolver) ResolveDeprecated(allowEmpty bool) ([]byte, error) {
	vars := boshtemplate.StaticVariables{}
	// TODO: old-style template parameters require very careful handling and reverse
	// order processing. we should eventually drop their support
	for i := len(resolver.params) - 1; i >= 0; i-- {
		if staticVar, ok := resolver.params[i].(boshtemplate.StaticVariables); ok {
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
