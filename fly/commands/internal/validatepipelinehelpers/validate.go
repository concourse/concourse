package validatepipelinehelpers

import (
	"errors"
	"fmt"

	"github.com/concourse/concourse/atc"

	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"sigs.k8s.io/yaml"
)

func Validate(yamlTemplate templatehelpers.YamlTemplateWithParams, strict bool, output bool, enableAcrossStep bool) error {
	evaluatedTemplate, err := yamlTemplate.Evaluate(strict)
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

	if enableAcrossStep {
		atc.EnableAcrossStep = true
	}

	configErrors, errorMessages := configvalidate.Validate(unmarshalledTemplate)

	var configErrorMessages []string
	for _, configError := range configErrors {
		configErrorMessages = append(configErrorMessages, configError.Message)
	}

	if len(configErrorMessages) > 0 {
		displayhelpers.ShowErrors("Error loading existing config", configErrorMessages)
		return fmt.Errorf("configuration invalid")
	}

	if len(errorMessages) > 0 {
		displayhelpers.ShowErrors("Error loading existing config", errorMessages)
		return fmt.Errorf("invalid configuration")
	}

	if len(errorMessages) > 0 || (strict && len(configErrors) > 0) {
		return errors.New("configuration invalid")
	}

	if output {
		fmt.Println(string(evaluatedTemplate))
	} else {
		fmt.Println("looks good")
	}

	return nil
}
