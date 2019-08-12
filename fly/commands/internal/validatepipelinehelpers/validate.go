package validatepipelinehelpers

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/ghodss/yaml"
)

func Validate(yamlTemplate templatehelpers.YamlTemplateWithParams, strict bool, output bool) error {
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
