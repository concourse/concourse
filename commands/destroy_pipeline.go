package commands

import (
	"fmt"

	"github.com/concourse/fly/rc"
	"github.com/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type DestroyPipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
}

func (command *DestroyPipelineCommand) Execute(args []string) error {
	pipelineName := command.Pipeline

	fmt.Printf("!!! this will remove all data for pipeline `%s`\n\n", pipelineName)

	confirm := false
	err := interact.NewInteraction("are you sure?").Resolve(&confirm)
	if err != nil || !confirm {
		fmt.Println("bailing out")
		return err
	}

	connection, err := rc.TargetConnection(Fly.Target)
	if err != nil {
		return err
	}

	client := concourse.NewClient(connection)

	found, err := client.DeletePipeline(pipelineName)
	if err != nil {
		return err
	}

	if !found {
		fmt.Printf("`%s` does not exist\n", pipelineName)
	} else {
		fmt.Printf("`%s` deleted\n", pipelineName)
	}

	return nil
}
