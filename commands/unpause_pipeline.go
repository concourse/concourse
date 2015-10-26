package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
)

type UnpausePipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to unpause"`
}

var unpausePipelineCommand UnpausePipelineCommand

func init() {
	configure, err := Parser.AddCommand(
		"unpause-pipeline",
		"Unpauses pipeline",
		"",
		&unpausePipelineCommand,
	)
	if err != nil {
		panic(err)
	}

	configure.Aliases = []string{"up"}
}

func (command *UnpausePipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	client, err := rc.TargetClient(globalOptions.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	handler := atcclient.NewAtcHandler(client)
	found, err := handler.UnpausePipeline(pipelineName)
	if err != nil {
		return err
	}

	if found {
		fmt.Printf("unpaused '%s'\n", pipelineName)
	} else {
		failf("pipeline '%s' not found\n", pipelineName)
	}
	return nil
}
