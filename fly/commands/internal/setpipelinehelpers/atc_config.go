package setpipelinehelpers

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	yaml "gopkg.in/yaml.v2"

	"github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	temp "github.com/concourse/fly/template"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/onsi/gomega/gexec"
	"github.com/vito/go-interact/interact"
)

type ATCConfig struct {
	PipelineName     string
	Team             concourse.Team
	Target           string
	SkipInteraction  bool
	CheckCredentials bool
}

func (atcConfig ATCConfig) ApplyConfigInteraction() bool {
	if atcConfig.SkipInteraction {
		return true
	}

	confirm := false
	err := interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil {
		return false
	}

	return confirm
}

func (atcConfig ATCConfig) Validate(
	configPath atc.PathFlag,
	templateVariables []flaghelpers.VariablePairFlag,
	yamlTemplateVariables []flaghelpers.YAMLVariablePairFlag,
	templateVariablesFiles []atc.PathFlag,
	strict bool,
	output bool,
) error {
	newConfig, err := atcConfig.newConfig(configPath, templateVariablesFiles, templateVariables, yamlTemplateVariables, true, strict)
	if err != nil {
		return err
	}

	var new atc.Config
	if strict {
		// UnmarshalStrict will pick up fields in structs that have the wrong names, as well as any duplicate keys in maps
		// we should consider always using this everywhere in a later release...
		if err := yaml.UnmarshalStrict([]byte(newConfig), &new); err != nil {
			return err
		}
	} else {
		if err := yaml.Unmarshal([]byte(newConfig), &new); err != nil {
			return err
		}
	}

	warnings, errorMessages := new.Validate()

	if len(warnings) > 0 {
		configWarnings := make([]concourse.ConfigWarning, len(warnings))
		for idx, warning := range warnings {
			configWarnings[idx] = concourse.ConfigWarning(warning)
		}
		atcConfig.showWarnings(configWarnings)
	}

	if len(errorMessages) > 0 {
		atcConfig.showPipelineConfigErrors(errorMessages)
	}

	if len(errorMessages) > 0 || (strict && len(warnings) > 0) {
		displayhelpers.Failf("configuration invalid")
	}

	if output {
		fmt.Println(string(newConfig))
	} else {
		fmt.Println("looks good")
	}

	return nil
}

func (atcConfig ATCConfig) Set(configPath atc.PathFlag, templateVariables []flaghelpers.VariablePairFlag, yamlTemplateVariables []flaghelpers.YAMLVariablePairFlag, templateVariablesFiles []atc.PathFlag) error {
	newConfig, err := atcConfig.newConfig(configPath, templateVariablesFiles, templateVariables, yamlTemplateVariables, false, false)
	if err != nil {
		return err
	}
	existingConfig, _, existingConfigVersion, _, err := atcConfig.Team.PipelineConfig(atcConfig.PipelineName)
	errorMessages := []string{}
	if err != nil {
		if configError, ok := err.(concourse.PipelineConfigError); ok {
			errorMessages = configError.ErrorMessages
		} else {
			return err
		}
	}

	var new atc.Config
	err = yaml.Unmarshal([]byte(newConfig), &new)
	if err != nil {
		return err
	}

	diffExists := diff(existingConfig, new)

	if len(errorMessages) > 0 {
		atcConfig.showPipelineConfigErrors(errorMessages)
	}

	if !diffExists {
		fmt.Println("no changes to apply")
		return nil
	}

	if !atcConfig.ApplyConfigInteraction() {
		fmt.Println("bailing out")
		return nil
	}

	created, updated, warnings, err := atcConfig.Team.CreateOrUpdatePipelineConfig(
		atcConfig.PipelineName,
		existingConfigVersion,
		newConfig,
		atcConfig.CheckCredentials,
	)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		atcConfig.showWarnings(warnings)
	}

	atcConfig.showHelpfulMessage(created, updated)
	return nil
}

func (atcConfig ATCConfig) newConfig(
	configPath atc.PathFlag,
	templateVariablesFiles []atc.PathFlag,
	templateVariables []flaghelpers.VariablePairFlag,
	yamlTemplateVariables []flaghelpers.YAMLVariablePairFlag,
	allowEmpty bool,
	strict bool,
) ([]byte, error) {
	evaluatedConfig, err := ioutil.ReadFile(string(configPath))
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %s", err.Error())
	}

	if strict {
		// We use a generic map here, since templates are not evaluated yet.
		// (else a template string may cause an error when a struct is expected)
		// If we don't check Strict now, then the subsequent steps will mask any
		// duplicate key errors.
		// We should consider being strict throughout the entire stack by default.
		err = yaml.UnmarshalStrict(evaluatedConfig, make(map[string]interface{}))
		if err != nil {
			return nil, fmt.Errorf("error parsing yaml before applying templates: %s", err.Error())
		}
	}

	var paramPayloads [][]byte
	for _, path := range templateVariablesFiles {
		templateVars, err := ioutil.ReadFile(string(path))
		if err != nil {
			return nil, fmt.Errorf("could not read template variables file (%s): %s", string(path), err.Error())
		}

		paramPayloads = append(paramPayloads, templateVars)
	}

	if temp.Present(evaluatedConfig) {
		evaluatedConfig, err = atcConfig.resolveDeprecatedTemplateStyle(evaluatedConfig, paramPayloads, templateVariables, yamlTemplateVariables, allowEmpty)
		if err != nil {
			return nil, fmt.Errorf("could not resolve old-style template vars: %s", err.Error())
		}
	}

	evaluatedConfig, err = atcConfig.resolveTemplates(evaluatedConfig, paramPayloads, templateVariables, yamlTemplateVariables)
	if err != nil {
		return nil, fmt.Errorf("could not resolve template vars: %s", err.Error())
	}

	return evaluatedConfig, nil
}

func (atcConfig ATCConfig) resolveTemplates(configPayload []byte, paramPayloads [][]byte, variables []flaghelpers.VariablePairFlag, yamlVariables []flaghelpers.YAMLVariablePairFlag) ([]byte, error) {
	tpl := template.NewTemplate(configPayload)

	flagVars := template.StaticVariables{}
	for _, f := range variables {
		flagVars[f.Name] = f.Value
	}

	for _, f := range yamlVariables {
		flagVars[f.Name] = f.Value
	}

	vars := []template.Variables{flagVars}
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

func (atcConfig ATCConfig) resolveDeprecatedTemplateStyle(
	configPayload []byte,
	paramPayloads [][]byte,
	variables []flaghelpers.VariablePairFlag,
	yamlVariables []flaghelpers.YAMLVariablePairFlag,
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

	flagVars := temp.Variables{}
	for _, flag := range variables {
		flagVars[flag.Name] = flag.Value
	}

	vars = vars.Merge(flagVars)

	return temp.Evaluate(configPayload, vars, allowEmpty)
}

func (atcConfig ATCConfig) showPipelineConfigErrors(errorMessages []string) {
	fmt.Fprintln(ui.Stderr, "")
	displayhelpers.PrintWarningHeader()

	fmt.Fprintln(ui.Stderr, "Error loading existing config:")
	for _, errorMessage := range errorMessages {
		fmt.Fprintf(ui.Stderr, "  - %s\n", errorMessage)
	}

	fmt.Fprintln(ui.Stderr, "")
}

func (atcConfig ATCConfig) showWarnings(warnings []concourse.ConfigWarning) {
	fmt.Fprintln(ui.Stderr, "")
	displayhelpers.PrintDeprecationWarningHeader()

	for _, warning := range warnings {
		fmt.Fprintf(ui.Stderr, "  - %s\n", warning.Message)
	}

	fmt.Fprintln(ui.Stderr, "")
}

func (atcConfig ATCConfig) showHelpfulMessage(created bool, updated bool) {
	if updated {
		fmt.Println("configuration updated")
	} else if created {

		targetURL, err := url.Parse(atcConfig.Target)
		if err != nil {
			fmt.Println("Could not parse targetURL")
		}

		pipelineURL, err := url.Parse("/teams/" + atcConfig.Team.Name() + "/pipelines/" + atcConfig.PipelineName)
		if err != nil {
			fmt.Println("Could not parse pipelineURL")
		}

		fmt.Println("pipeline created!")
		fmt.Printf("you can view your pipeline here: %s\n", targetURL.ResolveReference(pipelineURL))
		fmt.Println("")
		fmt.Println("the pipeline is currently paused. to unpause, either:")
		fmt.Println("  - run the unpause-pipeline command")
		fmt.Println("  - click play next to the pipeline in the web ui")
	} else {
		panic("Something really went wrong!")
	}
}

func diff(existingConfig atc.Config, newConfig atc.Config) bool {
	var diffExists bool

	indent := gexec.NewPrefixedWriter("  ", os.Stdout)

	groupDiffs := diffIndices(GroupIndex(existingConfig.Groups), GroupIndex(newConfig.Groups))
	if len(groupDiffs) > 0 {
		diffExists = true
		fmt.Println("groups:")

		for _, diff := range groupDiffs {
			diff.Render(indent, "group")
		}
	}

	resourceDiffs := diffIndices(ResourceIndex(existingConfig.Resources), ResourceIndex(newConfig.Resources))
	if len(resourceDiffs) > 0 {
		diffExists = true
		fmt.Println("resources:")

		for _, diff := range resourceDiffs {
			diff.Render(indent, "resource")
		}
	}

	resourceTypeDiffs := diffIndices(ResourceTypeIndex(existingConfig.ResourceTypes), ResourceTypeIndex(newConfig.ResourceTypes))
	if len(resourceTypeDiffs) > 0 {
		diffExists = true
		fmt.Println("resource types:")

		for _, diff := range resourceTypeDiffs {
			diff.Render(indent, "resource type")
		}
	}

	jobDiffs := diffIndices(JobIndex(existingConfig.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		diffExists = true
		fmt.Println("jobs:")

		for _, diff := range jobDiffs {
			diff.Render(indent, "job")
		}
	}
	return diffExists
}
