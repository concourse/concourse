package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type OrderInstancePipelinesCommand struct {
	Pipelines []flaghelpers.PipelineFlag `short:"p"  long:"pipeline" required:"true" description:"Pipeline to destroy"`
	Team      string                     `long:"team" description:"Name of the team to which the pipelines belong, if different from the target default"`
}

func (command *OrderInstancePipelinesCommand) Execute(args []string) error {
	for _, pipeline := range command.Pipelines {
		_, err := pipeline.Validate()
		if err != nil {
			return err
		}
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

	var pipelineRefs []atc.PipelineRef

	for _, pipeline := range command.Pipelines {
		pipelineRefs = append(pipelineRefs, pipeline.Ref())
	}

	err = team.OrderingPipelinesWithinGroup(pipelineRefs)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order instance pipelines", err)
	}

	fmt.Printf("ordered instance pipelines \n")
	for _, p := range pipelineRefs {
		fmt.Printf("  - %s \n", p.InstanceVars)
	}

	return nil
}
