package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/fly/ui"
	"github.com/mattn/go-isatty"
)

type GetPipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON     bool                     `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
}

func (command *GetPipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
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

	config, _, found, err := target.Team().PipelineConfig(pipelineName)
	if err != nil {
		return err
	}

	if !found {
		return errors.New("pipeline not found")
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

func (command *GetPipelineCommand) showConfigWarning() {
	if isatty.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(ui.Stderr, "")
	}
	displayhelpers.PrintWarningHeader()
	fmt.Fprintln(ui.Stderr, "Existing config is invalid, it was returned as-is")
}
