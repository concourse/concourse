package setpipelinehelpers

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/vito/go-interact/interact"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/configvalidate"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type ATCConfig struct {
	PipelineRef      atc.PipelineRef
	Team             concourse.Team
	TargetName       rc.TargetName
	Target           string
	SkipInteraction  bool
	CheckCredentials bool
	CommandWarnings  []concourse.ConfigWarning
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

func (atcConfig ATCConfig) Set(yamlTemplateWithParams templatehelpers.YamlTemplateWithParams) error {
	evaluatedTemplate, err := yamlTemplateWithParams.Evaluate(false, false)
	if err != nil {
		return err
	}

	existingConfig, existingConfigVersion, _, err := atcConfig.Team.PipelineConfig(atcConfig.PipelineRef)
	if err != nil {
		return err
	}

	var newConfig atc.Config
	err = yaml.Unmarshal([]byte(evaluatedTemplate), &newConfig)
	if err != nil {
		return err
	}

	configWarnings, _ := configvalidate.Validate(newConfig)
	for _, w := range configWarnings {
		atcConfig.CommandWarnings = append(atcConfig.CommandWarnings, concourse.ConfigWarning{
			Type:    w.Type,
			Message: w.Message,
		})
	}

	diffExists := diff(existingConfig, newConfig)

	if len(atcConfig.CommandWarnings) > 0 {
		displayhelpers.ShowWarnings(atcConfig.CommandWarnings)
	}

	if !diffExists {
		fmt.Println("no changes to apply")
		return nil
	}

	fmt.Println(bold("pipeline name: ") + atcConfig.PipelineRef.Name)
	if len(atcConfig.PipelineRef.InstanceVars) != 0 {
		fmt.Println(bold("pipeline instance vars:"))
		instanceVarsBytes, err := yaml.Marshal(atcConfig.PipelineRef.InstanceVars)
		if err != nil {
			return err
		}
		fmt.Println(indent(string(instanceVarsBytes), "  "))
	}
	fmt.Println()

	if !atcConfig.ApplyConfigInteraction() {
		fmt.Println("bailing out")
		return nil
	}

	created, updated, warnings, err := atcConfig.Team.CreateOrUpdatePipelineConfig(
		atcConfig.PipelineRef,
		existingConfigVersion,
		evaluatedTemplate,
		atcConfig.CheckCredentials,
	)
	if err != nil {
		return err
	}

	updatedPipeline, _, err := atcConfig.Team.Pipeline(atcConfig.PipelineRef)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	atcConfig.showPipelineUpdateResult(updatedPipeline, created, updated)
	return nil
}

func (atcConfig ATCConfig) UnpausePipelineCommand() string {
	pipelineFlag := atcConfig.PipelineRef.String()
	if strings.Contains(pipelineFlag, `"`) {
		pipelineFlag = strconv.Quote(pipelineFlag)
	}
	return fmt.Sprintf("%s -t %s unpause-pipeline -p %s", os.Args[0], atcConfig.TargetName, pipelineFlag)
}

func (atcConfig ATCConfig) showPipelineUpdateResult(pipeline atc.Pipeline, created bool, updated bool) {
	if updated {
		fmt.Println("configuration updated")
	} else if created {
		targetURL, err := url.Parse(atcConfig.Target)
		if err != nil {
			fmt.Println("Could not parse targetURL")
		}

		queryParams := atcConfig.PipelineRef.QueryParams().Encode()
		if queryParams != "" {
			queryParams = "?" + queryParams
		}
		pipelineURL, err := url.Parse("/teams/" + atcConfig.Team.Name() + "/pipelines/" + atcConfig.PipelineRef.Name + queryParams)
		if err != nil {
			fmt.Println("Could not parse pipelineURL")
		}

		fmt.Println("pipeline created!")
		fmt.Printf("you can view your pipeline here: %s\n", targetURL.ResolveReference(pipelineURL))
	} else {
		panic("Something really went wrong!")
	}

	if pipeline.Paused {
		fmt.Println("")
		fmt.Println("the pipeline is currently paused. to unpause, either:")
		fmt.Println("  - run the unpause-pipeline command:")
		fmt.Println("    " + atcConfig.UnpausePipelineCommand())
		fmt.Println("  - click play next to the pipeline in the web ui")
	}
}

func diff(existingConfig atc.Config, newConfig atc.Config) bool {
	stdout, _ := ui.ForTTY(os.Stdout)
	return existingConfig.Diff(stdout, newConfig)
}

func indent(text, indent string) string {
	text = strings.TrimRight(text, "\n")
	var result strings.Builder
	for _, line := range strings.Split(text, "\n") {
		result.WriteString(indent + line + "\n")
	}
	return result.String()[:result.Len()-1]
}

func bold(text string) string {
	return "\033[1m" + text + "\033[0m"
}
