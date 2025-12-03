package validatepipelinehelpers

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"

	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/goccy/go-yaml"
)

func Validate(yamlTemplate templatehelpers.YamlTemplateWithParams, strict bool, output bool) error {
	evaluatedTemplate, err := yamlTemplate.Evaluate(true)
	if err != nil {
		return err
	}

	var unmarshalledTemplate atc.Config
	if strict {
		// additionally catches unknown keys
		err = yaml.UnmarshalWithOptions(evaluatedTemplate, &unmarshalledTemplate, yaml.UseJSONUnmarshaler(), yaml.Strict())
	} else {
		err = yaml.UnmarshalWithOptions(evaluatedTemplate, &unmarshalledTemplate, yaml.UseJSONUnmarshaler())
	}
	if err != nil {
		return err
	}

	warnings, errorMessages := configvalidate.Validate(unmarshalledTemplate)

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
		return errors.New("configuration invalid")
	}

	if output {
		fmt.Println(string(evaluatedTemplate))
	} else {
		fmt.Println("looks good")
	}

	return nil
}
