package setpipelinehelpers

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/atc/web"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/template"
	"github.com/concourse/go-concourse/concourse"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/rata"
	"github.com/vito/go-interact/interact"
	"gopkg.in/yaml.v2"
)

type ATCConfig struct {
	PipelineName        string
	Client              concourse.Client
	WebRequestGenerator *rata.RequestGenerator
}

func (atcConfig ATCConfig) Set(configPath flaghelpers.PathFlag, templateVariables template.Variables, templateVariablesFiles []flaghelpers.PathFlag) {
	newConfig := atcConfig.newConfig(configPath, templateVariablesFiles, templateVariables)
	existingConfig, existingConfigVersion, _, err := atcConfig.Client.PipelineConfig(atcConfig.PipelineName)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to retrieve config", err)
	}

	diff(existingConfig, newConfig)

	created, updated, err := atcConfig.Client.CreateOrUpdatePipelineConfig(
		atcConfig.PipelineName,
		existingConfigVersion,
		newConfig,
	)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to update configuration", err)
	}
	atcConfig.showHelpfulMessage(created, updated)
}

func (atcConfig ATCConfig) newConfig(configPath flaghelpers.PathFlag, templateVariablesFiles []flaghelpers.PathFlag, templateVariables template.Variables) atc.Config {
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

	var newConfig atc.Config
	err = yaml.Unmarshal(configFile, &newConfig)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to parse configuration file", err)
	}

	return newConfig
}

func (atcConfig ATCConfig) showHelpfulMessage(created bool, updated bool) {
	if updated {
		fmt.Println("configuration updated")
	} else if created {
		pipelineWebReq, _ := atcConfig.WebRequestGenerator.CreateRequest(
			web.Pipeline,
			rata.Params{"pipeline": atcConfig.PipelineName},
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

	jobDiffs := diffIndices(JobIndex(existingConfig.Jobs), JobIndex(newConfig.Jobs))
	if len(jobDiffs) > 0 {
		fmt.Println("jobs:")

		for _, diff := range jobDiffs {
			diff.Render(indent, "job")
		}
	}

	confirm := false
	err := interact.NewInteraction("apply configuration?").Resolve(&confirm)
	if err != nil || !confirm {
		fmt.Println("bailing out")
		os.Exit(1)
	}
}
