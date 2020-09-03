package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
	"github.com/concourse/concourse/go-concourse/concourse"
	"github.com/vito/go-interact/interact"
)

type DestroyPipelineCommand struct {
	Pipeline        flaghelpers.PipelineFlag `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
	SkipInteractive bool                     `short:"n"  long:"non-interactive"          description:"Destroy the pipeline without confirmation"`

	Team string `long:"team" description:"Name of the team to which the pipeline belongs, if different from the target default"`
}

func (command *DestroyPipelineCommand) Validate() error {
	_, err := command.Pipeline.Validate()
	return err
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

	var team concourse.Team

	if command.Team != "" {
		team, err = target.FindTeam(command.Team)
		if err != nil {
			return err
		}
	} else {
		team = target.Team()
	}

	pipelineRef := command.Pipeline.Ref()
	fmt.Printf("!!! this will remove all data for pipeline `%s`\n\n", pipelineRef.String())

	confirm := command.SkipInteractive
	if !confirm {
		err := interact.NewInteraction("are you sure?").Resolve(&confirm)
		if err != nil || !confirm {
			fmt.Println("bailing out")
			return err
		}
	}

	found, err := team.DeletePipeline(pipelineRef)
	if err != nil {
		return err
	}

	if !found {
		fmt.Printf("`%s` does not exist\n", pipelineRef.String())
	} else {
		fmt.Printf("`%s` deleted\n", pipelineRef.String())
	}

	return nil
}
