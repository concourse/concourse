package setpipelinehelpers

import (
	"fmt"
	"github.com/cloudfoundry/bosh-cli/director/template"
	atctemplate "github.com/concourse/concourse/atc/template"
	"io/ioutil"
	"net/url"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
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
	newConfig, err := atcConfig.NewConfig(configPath, templateVariablesFiles, templateVariables, yamlTemplateVariables, true, strict)
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
	newConfig, err := atcConfig.NewConfig(configPath, templateVariablesFiles, templateVariables, yamlTemplateVariables, false, false)
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

func (atcConfig ATCConfig) NewConfig(
	configPath atc.PathFlag,
	templateVariablesFiles []atc.PathFlag,
	templateVariables []flaghelpers.VariablePairFlag,
	yamlTemplateVariables []flaghelpers.YAMLVariablePairFlag,
	allowEmpty bool,
	strict bool,
) ([]byte, error) {
	config, err := ioutil.ReadFile(string(configPath))
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

	var paramPayloads [][]byte
	for _, path := range templateVariablesFiles {
		templateVars, err := ioutil.ReadFile(string(path))
		if err != nil {
			return nil, fmt.Errorf("could not read template variables file (%s): %s", string(path), err.Error())
		}

		paramPayloads = append(paramPayloads, templateVars)
	}

	flagVars := template.StaticVariables{}
	for _, f := range templateVariables {
		flagVars[f.Name] = f.Value
	}
	for _, f := range yamlTemplateVariables {
		flagVars[f.Name] = f.Value
	}

	evaluatedConfig, err := atctemplate.TemplateResolver{}.Resolve(config, paramPayloads, flagVars, allowEmpty)
	if err != nil {
		return nil, err
	}

	return evaluatedConfig, nil
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

	stdout, _ := ui.ForTTY(os.Stdout)

	indent := gexec.NewPrefixedWriter("  ", stdout)

	groupDiffs := groupDiffIndices(GroupIndex(existingConfig.Groups), GroupIndex(newConfig.Groups))
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
