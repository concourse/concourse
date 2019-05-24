package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type ExposePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" env:"PIPELINE" description:"Pipeline to expose"`
}

func (command *ExposePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *ExposePipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	found, err := target.Team().ExposePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("exposed '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
