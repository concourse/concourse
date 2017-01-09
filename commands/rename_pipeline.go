package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/fly/rc"
)

type RenamePipelineCommand struct {
	Pipeline flaghelpers.PipelineFlag `short:"o"  long:"old-name" required:"true"  description:"Pipeline to rename"`
	Name     string                   `short:"n"  long:"new-name" required:"true"  description:"Name to set as pipeline name"`
}

func (command *RenamePipelineCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	pipelineName := string(command.Pipeline)

	found, err := target.Team().RenamePipeline(pipelineName, command.Name)
	if err != nil {
		return err
	}

	if !found {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
		return nil
	}

	fmt.Printf("pipeline successfully renamed to %s\n", command.Name)

	return nil
}
