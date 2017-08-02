package commands

import (
	"fmt"

	"github.com/concourse/fly/rc"
	"github.com/vito/go-interact/interact"

	"github.com/concourse/fly/commands/internal/flaghelpers"
)

type DestroyPipelineCommand struct {
	Pipeline        flaghelpers.PipelineFlag `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
	SkipInteractive bool                     `short:"n"  long:"non-interactive"          description:"Destroy the pipeline without confirmation"`
}

func (command *DestroyPipelineCommand) Validate() error {
	return command.Pipeline.Validate()
}

func (command *DestroyPipelineCommand) Execute(args []string) error {
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

	pipelineName := string(command.Pipeline)
	fmt.Printf("!!! this will remove all data for pipeline `%s`\n\n", pipelineName)

	confirm := command.SkipInteractive
	if !confirm {
		err := interact.NewInteraction("are you sure?").Resolve(&confirm)
		if err != nil || !confirm {
			fmt.Println("bailing out")
			return err
		}
	}

	found, err := target.Team().DeletePipeline(pipelineName)
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
