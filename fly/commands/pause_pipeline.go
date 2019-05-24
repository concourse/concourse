package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type PausePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p"  long:"pipeline" required:"true" env:"PIPELINE" description:"Pipeline to pause"`
}

func (command *PausePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *PausePipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	found, err := target.Team().PausePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("paused '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
