package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
)

type PausePipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to pause"`
}

func (command *PausePipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	client := concourse.NewClient(connection)
	found, err := client.PausePipeline(pipelineName)
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
