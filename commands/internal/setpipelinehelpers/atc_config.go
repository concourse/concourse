package setpipelinehelpers

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/template"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/mitchellh/mapstructure"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/rata"
	"github.com/vito/go-interact/interact"
	"gopkg.in/yaml.v2"
)

type ATCConfig struct {
	PipelineName        string
	Team                concourse.Team
	WebRequestGenerator *rata.RequestGenerator
	SkipInteraction     bool
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

func (atcConfig ATCConfig) Set(configPath atc.PathFlag, templateVariables template.Variables, templateVariablesFiles []atc.PathFlag) error {
	newConfig := atcConfig.newConfig(configPath, templateVariablesFiles, templateVariables)
	existingConfig, _, existingConfigVersion, _, err := atcConfig.Team.PipelineConfig(atcConfig.PipelineName)
	errorMessages := []string{}
	if err != nil {
		if configError, ok := err.(concourse.PipelineConfigError); ok {
			errorMessages = configError.ErrorMessages
		} else {
			return err
		}
	}

	diff(existingConfig, newConfig)

	if len(errorMessages) > 0 {
		atcConfig.showPipelineConfigErrors(errorMessages)
	}

	if !atcConfig.ApplyConfigInteraction() {
		fmt.Println("bailing out")
		return nil
	}

	created, updated, warnings, err := atcConfig.Team.CreateOrUpdatePipelineConfig(
		atcConfig.PipelineName,
		existingConfigVersion,
		newConfig,
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

func (atcConfig ATCConfig) newConfig(configPath atc.PathFlag, templateVariablesFiles []atc.PathFlag, templateVariables template.Variables) atc.Config {
	configFile, err := ioutil.ReadFile(string(configPath))
	if err != nil {
		displayhelpers.FailWithErrorf("could not read config file", err)
	}

	var resultVars template.Variables

	for _, path := range templateVariablesFiles {
		fileVars, templateErr := template.LoadVariablesFromFile(string(path))
		if templateErr != nil {
			displayhelpers.FailWithErrorf("failed to load variables from file (%s)", templateErr, string(path))
		}

		resultVars = resultVars.Merge(fileVars)
	}

	resultVars = resultVars.Merge(templateVariables)

	configFile, err = template.Evaluate(configFile, resultVars)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to evaluate variables into template", err)
	}

	var configStructure interface{}
	err = yaml.Unmarshal(configFile, &configStructure)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to unmarshal configStructure", err)
	}

	var newConfig atc.Config
	msConfig := &mapstructure.DecoderConfig{
		Result:           &newConfig,
		WeaklyTypedInput: true,
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			atc.SanitizeDecodeHook,
			atc.VersionConfigDecodeHook,
		),
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to construct decoder", err)
	}

	if err := decoder.Decode(configStructure); err != nil {
		displayhelpers.FailWithErrorf("failed to decode config", err)
	}

	return newConfig
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
		pipelineWebReq, _ := atcConfig.WebRequestGenerator.CreateRequest(
			web.Pipeline,
			rata.Params{
				"pipeline":  atcConfig.PipelineName,
				"team_name": atcConfig.Team.Name(),
			},
			nil,
		)

		fmt.Println("pipeline created!")

		pipelineURL := pipelineWebReq.URL
		// don't show username and password
		pipelineURL.User = nil

		fmt.Printf("you can view your pipeline here: %s\n", pipelineURL.String())

		fmt.Println("")
		fmt.Println("the pipeline is currently paused. to unpause, either:")
		fmt.Println("  - run the unpause-pipeline command")
		fmt.Println("  - click play next to the pipeline in the web ui")
	} else {
		panic("Something really went wrong!")
	}
}

func diff(existingConfig atc.Config, newConfig atc.Config) {
	indent := gexec.NewPrefixedWriter("  ", os.Stdout)

	groupDiffs := diffIndices(GroupIndex(existingConfig.Groups), GroupIndex(newConfig.Groups))
	if len(groupDiffs) > 0 {
		fmt.Println("groups:")

		for _, diff := range groupDiffs {
			diff.Render(indent, "group")
		}
	}

	resourceDiffs := diffIndices(ResourceIndex(existingConfig.Resources), ResourceIndex(newConfig.Resources))
	if len(resourceDiffs) > 0 {
		fmt.Println("resources:")

		for _, diff := range resourceDiffs {
			diff.Render(indent, "resource")
		}
	}

	resourceTypeDiffs := diffIndices(ResourceTypeIndex(existingConfig.ResourceTypes), ResourceTypeIndex(newConfig.ResourceTypes))
	if len(resourceTypeDiffs) > 0 {
		fmt.Println("resource types:")

		for _, diff := range resourceTypeDiffs {
			diff.Render(indent, "resource type")
		}
	}

	jobDiffs := diffIndices(JobIndex(existingConfig.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		fmt.Println("jobs:")

		for _, diff := range jobDiffs {
			diff.Render(indent, "job")
		}
	}
}
