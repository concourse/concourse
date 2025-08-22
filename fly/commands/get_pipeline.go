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
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/mattn/go-isatty"
)

type GetPipelineCommand struct {
	Pipeline       flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Get configuration of this pipeline"`
	JSON           bool                     `short:"j" long:"json"                     description:"Print config as json instead of yaml"`
	Team           flaghelpers.TeamFlag     `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
	SkipValidation bool                     `long:"skip-validation" required:"false" description:"Skip identifier validation before destroying"`
}

func (command *GetPipelineCommand) Validate() error {
	err := command.Pipeline.Validate()
	return err
}

func (command *GetPipelineCommand) Execute(args []string) error {
	skip_validation := command.SkipValidation
	if !skip_validation {
		err := command.Validate()
		if err != nil {
			return err
		}
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	var team concourse.Team
	team, err = command.Team.LoadTeam(target)
	if err != nil {
		return err
	}

	config, _, found, err := team.PipelineConfig(command.Pipeline.Ref())
	if err != nil {
		return err
	}

	if !found {
		return errors.New("pipeline not found")
	}

	return dump(config, command.JSON)
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
