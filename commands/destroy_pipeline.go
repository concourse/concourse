package commands

import (
	"fmt"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
	"github.com/vito/go-interact/interact"
)

type DestroyPipelineCommand struct {
	Pipeline string `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
}

var destroyPipelineCommand DestroyPipelineCommand

func init() {
	destroyPipeline, err := Parser.AddCommand(
		"destroy-pipeline",
		"Destroy a pipeline",
		"",
		&destroyPipelineCommand,
	)
	if err != nil {
		panic(err)
	}

	destroyPipeline.Aliases = []string{"d"}
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

	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		return err
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		return err
	}

	handler := atcclient.NewAtcHandler(client)

	found, err := handler.DeletePipeline(pipelineName)
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
