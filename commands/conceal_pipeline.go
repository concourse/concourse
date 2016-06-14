package commands

import (
	"fmt"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type ConcealPipelineCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Pipeline to conceal"`
}

func (command *ConcealPipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	target, err := rc.LoadTarget(Fly.Target)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	found, err := target.Team().ConcealPipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("concealed '%s'\n", pipelineName)
	} else {
		displayhelpers.Failf("pipeline '%s' not found\n", pipelineName)
	}

	return nil
}
