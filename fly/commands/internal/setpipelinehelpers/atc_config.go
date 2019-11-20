package setpipelinehelpers

import (
	"fmt"
	"net/url"
	"os"
	"sigs.k8s.io/yaml"

	"github.com/onsi/gomega/gexec"
	"github.com/vito/go-interact/interact"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/templatehelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/concourse/concourse/go-concourse/concourse"
)

type ATCConfig struct {
	PipelineName     string
	Team             concourse.Team
	TargetName       rc.TargetName
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

func (atcConfig ATCConfig) Set(yamlTemplateWithParams templatehelpers.YamlTemplateWithParams) error {
	evaluatedTemplate, err := yamlTemplateWithParams.Evaluate(false, false)
	if err != nil {
		return err
	}

	existingConfig, existingConfigVersion, _, err := atcConfig.Team.PipelineConfig(atcConfig.PipelineName)
	if err != nil {
		return err
	}

	var newConfig atc.Config
	err = yaml.Unmarshal([]byte(evaluatedTemplate), &newConfig)
	if err != nil {
		return err
	}

	diffExists := diff(existingConfig, newConfig)

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
		evaluatedTemplate,
		atcConfig.CheckCredentials,
	)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	atcConfig.showPipelineUpdateResult(created, updated)
	return nil
}

func (atcConfig ATCConfig) UnpausePipelineCommand() string {
	return fmt.Sprintf("%s -t %s unpause-pipeline -p %s", os.Args[0], atcConfig.TargetName, atcConfig.PipelineName)
}

func (atcConfig ATCConfig) showPipelineUpdateResult(created bool, updated bool) {
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
		fmt.Println("  - run the unpause-pipeline command:")
		fmt.Println("    " + atcConfig.UnpausePipelineCommand())
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

	varSourceDiffs := diffIndices(VarSourceIndex(existingConfig.VarSources), VarSourceIndex(newConfig.VarSources))
	if len(varSourceDiffs) > 0 {
		diffExists = true
		fmt.Println("variable source:")

		for _, diff := range varSourceDiffs {
			diff.Render(indent, "variable source")
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
