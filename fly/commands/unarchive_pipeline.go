package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type UnarchivePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Pipeline to unarchive"`
}

func (command *UnarchivePipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *UnarchivePipelineCommand) Execute(args []string) error {
	err := command.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().UnarchivePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("unarchived '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
