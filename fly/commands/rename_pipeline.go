package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type RenamePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"o" long:"old-name" required:"true" env:"PIPELINE" description:"Pipeline to rename"`
	Name     flaghelpers.PipelineFlag `short:"n" long:"new-name" required:"true"                description:"Name to set as pipeline name"`
}

func (command *RenamePipelineCommand) Validate() error {
	err := command.Pipeline.Validate()
	if err != nil {
		return err
	}

	return command.Name.Validate()
}

func (command *RenamePipelineCommand) Execute([]string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	oldName := string(command.Pipeline)
	newName := string(command.Name)

	found, err := target.Team().RenamePipeline(oldName, newName)
	if err != nil {
		return err
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", oldName)
		return nil
	}

	fmt.Printf("pipeline successfully renamed to %s\n", newName)

	return nil
}
