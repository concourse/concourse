package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/yaml.v2"

	"github.com/concourse/atc"
	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/fly/ui"
	"github.com/concourse/go-concourse/concourse"
	"github.com/mattn/go-isatty"
)

type GetPipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON     bool                     `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
}

func (command *GetPipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *GetPipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	asJSON := command.JSON
	pipelineName := string(command.Pipeline)

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	config, rawConfig, _, _, err := target.Team().PipelineConfig(pipelineName)
	if err != nil {
		if _, ok := err.(concourse.PipelineConfigError); ok {
			dumpRawConfig(rawConfig, asJSON)
			command.showConfigWarning()
			return err
		} else {
			return err
		}
	}

	return dump(config, asJSON)
}

func dump(config atc.Config, asJSON bool) error {
	var payload []byte
	var err error
	if asJSON {
		payload, err = json.Marshal(config)
	} else {
		payload, err = yaml.Marshal(config)
	}
	if err != nil {
		return err
	}

	_, err = fmt.Printf("%s", payload)

	return err
}

func dumpRawConfig(rawConfig atc.RawConfig, asJSON bool) error {
	var payload []byte
	if asJSON {
		payload = []byte(rawConfig)
	} else {
		var config map[string]interface{}
		err := json.Unmarshal([]byte(rawConfig), &config)
		if err != nil {
			return err
		}

		payload, err = yaml.Marshal(config)
		if err != nil {
			return err
		}
	}

	_, err := fmt.Printf("%s", payload)
	if err != nil {
		return err
	}

	return nil
}

func (command *GetPipelineCommand) showConfigWarning() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(ui.Stderr, "")
	}
	displayhelpers.PrintWarningHeader()
	fmt.Fprintln(ui.Stderr, "Existing config is invalid, it was returned as-is")
}
