package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type UnpausePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" env:"PIPELINE" description:"Pipeline to unpause"`
}

func (command *UnpausePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *UnpausePipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	found, err := target.Team().UnpausePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("unpaused '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
