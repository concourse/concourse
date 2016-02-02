package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
)

type UnpausePipelineCommand struct {
	Pipeline string `short:"p" long:"pipeline" required:"true" description:"Pipeline to unpause"`
}

func (command *UnpausePipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	found, err := client.UnpausePipeline(pipelineName)
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
