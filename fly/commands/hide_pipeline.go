package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
)

type HidePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Pipeline to hide"`
}

func (command *HidePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *HidePipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	target, err := Fly.RetrieveTarget()
	if err != nil {
		return err
	}

	found, err := target.Team().HidePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("hid '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
