package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
)

type PausePipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to pause"`
}

func (command *PausePipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	client, err := rc.TargetClient(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	handler := atcclient.NewAtcHandler(client)
	found, err := handler.PausePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("paused '%s'\n", pipelineName)
	} else {
		failf("pipeline '%s' not found\n", pipelineName)
	}
	return nil
}
