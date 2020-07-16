package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type RenamePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"o"  long:"old-name" required:"true"  description:"Pipeline to rename"`
	Name     flaghelpers.PipelineFlag `short:"n"  long:"new-name" required:"true"  description:"Name to set as pipeline name"`
}

func (command *RenamePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	if err != nil {
		return err
	}

	_, err = command.Name.Validate()
	return err
}

func (command *RenamePipelineCommand) Execute([]string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	oldName := string(command.Pipeline)
	newName := string(command.Name)

	found, warnings, err := target.Team().RenamePipeline(oldName, newName)
	if err != nil {
		return err
	}

	if len(warnings) > 0 {
		displayhelpers.ShowWarnings(warnings)
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", oldName)
		return nil
	}

	fmt.Printf("pipeline successfully renamed to %s\n", newName)

	return nil
}
