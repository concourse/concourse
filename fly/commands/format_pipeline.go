package commands

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
)

type FormatPipelineCommand struct {
	Config atc.PathFlag `short:"c" long:"config" required:"true" description:"Pipeline configuration file"`
	Write  bool         `short:"w" long:"write" description:"Do not print to stdout; overwrite the file in place"`
}

func (command *FormatPipelineCommand) Execute(args []string) error {
	configPath := string(command.Config)
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		displayhelpers.FailWithErrorf("could not read config file", err)
	}

	var config atc.Config
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		displayhelpers.FailWithErrorf("could not unmarshal config", err)
	}

	formattedBytes, err := yaml.Marshal(config)
	if err != nil {
		displayhelpers.FailWithErrorf("could not marshal config", err)
	}

	if command.Write {
		fi, err := os.Stat(configPath)
		if err != nil {
			displayhelpers.FailWithErrorf("could not stat config file", err)
		}

		err = os.WriteFile(configPath, formattedBytes, fi.Mode())
		if err != nil {
			displayhelpers.FailWithErrorf("could not write formatted config to %s", err, command.Config)
		}
	} else {
		_, err = fmt.Fprint(os.Stdout, string(formattedBytes))
		if err != nil {
			displayhelpers.FailWithErrorf("could not write formatted config to stdout", err)
		}
	}

	return nil
}
