package templatehelpers

import (
	"fmt"
	boshtemplate "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/template"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/go-concourse/concourse"

	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type YamlTemplateWithParams struct {
	filePath               atc.PathFlag
	templateVariablesFiles []atc.PathFlag
	templateVariables      []flaghelpers.VariablePairFlag
	yamlTemplateVariables  []flaghelpers.YAMLVariablePairFlag
}

func NewYamlTemplateWithParams(filePath atc.PathFlag, templateVariablesFiles []atc.PathFlag, templateVariables []flaghelpers.VariablePairFlag, yamlTemplateVariables []flaghelpers.YAMLVariablePairFlag) YamlTemplateWithParams {
	return YamlTemplateWithParams{
		filePath:               filePath,
		templateVariablesFiles: templateVariablesFiles,
		templateVariables:      templateVariables,
		yamlTemplateVariables:  yamlTemplateVariables,
	}
}

func (yamlTemplate YamlTemplateWithParams) Evaluate(
	allowEmpty bool,
	strict bool,
) ([]byte, error) {
	config, err := ioutil.ReadFile(string(yamlTemplate.filePath))
	if err != nil {
		return nil, fmt.Errorf("could not read file: %s", err.Error())
	}

	if strict {
		// We use a generic map here, since templates are not evaluated yet.
		// (else a template string may cause an error when a struct is expected)
		// If we don't check Strict now, then the subsequent steps will mask any
		// duplicate key errors.
		// We should consider being strict throughout the entire stack by default.
		err = yaml.UnmarshalStrict(config, make(map[string]interface{}))
		if err != nil {
			return nil, fmt.Errorf("error parsing yaml before applying templates: %s", err.Error())
		}
	}

	var params []boshtemplate.StaticVariables
	for _, path := range yamlTemplate.templateVariablesFiles {
		templateVars, err := ioutil.ReadFile(string(path))
		if err != nil {
			return nil, fmt.Errorf("could not read template variables file (%s): %s", string(path), err.Error())
		}

		var staticVars boshtemplate.StaticVariables
		err = yaml.Unmarshal(templateVars, &staticVars)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal template variables (%s): %s", string(path), err.Error())
		}

		params = append(params, staticVars)
	}

	vars := boshtemplate.StaticVariables{}
	for _, f := range yamlTemplate.templateVariables {
		vars[f.Name] = f.Value
	}
	for _, f := range yamlTemplate.yamlTemplateVariables {
		vars[f.Name] = f.Value
	}
	params = append(params, vars)

	evaluatedConfig, err := template.NewTemplateResolver(config, params).Resolve(allowEmpty)
	if err != nil {
		return nil, err
	}

	return evaluatedConfig, nil
}

func (yamlTemplate YamlTemplateWithParams) Validate(
	strict bool,
	output bool,
) error {
	evaluatedTemplate, err := yamlTemplate.Evaluate(true, strict)
	if err != nil {
		return err
	}

	var unmarshalledTemplate atc.Config
	if strict {
		// UnmarshalStrict will pick up fields in structs that have the wrong names, as well as any duplicate keys in maps
		// we should consider always using this everywhere in a later release...
		if err := yaml.UnmarshalStrict([]byte(evaluatedTemplate), &unmarshalledTemplate); err != nil {
			return err
		}
	} else {
		if err := yaml.Unmarshal([]byte(evaluatedTemplate), &unmarshalledTemplate); err != nil {
			return err
		}
	}

	warnings, errorMessages := unmarshalledTemplate.Validate()

	if len(warnings) > 0 {
		configWarnings := make([]concourse.ConfigWarning, len(warnings))
		for idx, warning := range warnings {
			configWarnings[idx] = concourse.ConfigWarning(warning)
		}
		displayhelpers.ShowWarnings(configWarnings)
	}

	if len(errorMessages) > 0 {
		displayhelpers.ShowErrors("Error loading existing config", errorMessages)
	}

	if len(errorMessages) > 0 || (strict && len(warnings) > 0) {
		displayhelpers.Failf("configuration invalid")
	}

	if output {
		fmt.Println(string(evaluatedTemplate))
	} else {
		fmt.Println("looks good")
	}

	return nil
}
