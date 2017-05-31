package commands

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/template"
	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v2"
)

type ValidatePipelineCommand struct {
	Config atc.PathFlag `short:"c" long:"config" required:"true"        description:"Pipeline configuration file"`
	Strict bool         `short:"s" long:"strict"                        description:"Fail on warnings"`
}

func (command *ValidatePipelineCommand) Execute(args []string) error {
	configPath := command.Config
	strict := command.Strict

	configFile, err := ioutil.ReadFile(string(configPath))
	if err != nil {
		displayhelpers.FailWithErrorf("could not read config file", err)
	}

	configFile = template.EvaluateEmpty(configFile)

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
			atc.LoadTaskConfigDecodeHook,
		),
	}

	decoder, err := mapstructure.NewDecoder(msConfig)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to construct decoder", err)
	}

	if err := decoder.Decode(configStructure); err != nil {
		displayhelpers.FailWithErrorf("failed to decode config", err)
	}

	warnings, errorMessages := newConfig.Validate()

	if len(warnings) > 0 {
		displayhelpers.PrintDeprecationWarningHeader()

		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "  - %s\n", warning.Message)
		}

		fmt.Fprintln(os.Stderr, "")
	}

	if len(errorMessages) > 0 {
		displayhelpers.PrintWarningHeader()

		for _, errorMessage := range errorMessages {
			fmt.Fprintf(os.Stderr, "  - %s\n", errorMessage)
		}

		fmt.Fprintln(os.Stderr, "")
	}

	if len(errorMessages) > 0 || (strict && len(warnings) > 0) {
		displayhelpers.Failf("configuration invalid")
	}

	fmt.Println("looks good")
	return nil
}
