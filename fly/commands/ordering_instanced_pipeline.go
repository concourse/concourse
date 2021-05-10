package commands

import (
	"fmt"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/go-concourse/concourse"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type OrderInstancedPipelinesCommand struct {
	Group        string                         `short:"g" long:"group" required:"true" description:"Name of the instance group"`
	InstanceVars []flaghelpers.InstanceVarsFlag `short:"p" long:"pipeline" required:"true" description:"Instance vars identifying pipeline (can be specified multiple times to provide relative ordering)"`
	Team         string                         `long:"team" description:"Name of the team to which the pipelines belong, if different from the target default"`
}

func (command *OrderInstancedPipelinesCommand) Execute(args []string) error {
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

	var instanceVars []atc.InstanceVars

	for _, instanceVar := range command.InstanceVars {
		instanceVars = append(instanceVars, instanceVar.InstanceVars)
	}

	err = team.OrderingPipelinesWithinGroup(command.Group, instanceVars)
	if err != nil {
		displayhelpers.FailWithErrorf("failed to order instanced pipelines", err)
	}

	fmt.Printf("ordered instanced pipelines \n")
	for _, iv := range instanceVars {
		fmt.Printf("  - %s \n", iv)
	}

	return nil
}
