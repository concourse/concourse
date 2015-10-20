package commands

import (
	"fmt"
	"log"

	"github.com/concourse/fly/atcclient"
	"github.com/concourse/fly/rc"
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

	if !askToConfirm("are you sure?") {
		log.Fatalln("bailing out")
	}

	target, err := rc.SelectTarget(globalOptions.Target, globalOptions.Insecure)
	if err != nil {
		log.Fatalln(err)
		return nil
	}

	client, err := atcclient.NewClient(*target)
	if err != nil {
		log.Fatalln("failed to create client:", err)
	}

	handler := atcclient.NewAtcHandler(client)
	found, err := handler.DeletePipeline(pipelineName)
	if err != nil {
		log.Fatalln(err)
	}
	if !found {
		log.Fatalln(fmt.Sprintf("`%s` does not exist", pipelineName))
	}

	fmt.Printf("`%s` deleted\n", pipelineName)
	return nil
}
