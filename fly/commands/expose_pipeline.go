package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type ExposePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"p" long:"pipeline" required:"true" description:"Pipeline to expose"`
}

func (command *ExposePipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
}

func (command *ExposePipelineCommand) Execute(args []string) error {
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

	pipelineRef := command.Pipeline.Ref()
	found, err := target.Team().ExposePipeline(pipelineRef)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("exposed '%s'\n", pipelineRef.String())
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineRef.String())
	}

	return nil
}
