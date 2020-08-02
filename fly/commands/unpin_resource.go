package commands

import (
	"fmt"

	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type UnpinResourceCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of the resource"`
}

func (command *UnpinResourceCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	team := target.Team()

	unpinned, err := team.UnpinResource(command.Resource.PipelineRef, command.Resource.ResourceName)
	if err != nil {
		return err
	}

	if unpinned {
		fmt.Printf("unpinned '%s/%s'\n", command.Resource.PipelineRef.String(), command.Resource.ResourceName)
	} else {
		displayhelpers.Failf("could not find resource '%s/%s'\n", command.Resource.PipelineRef.String(), command.Resource.ResourceName)
	}

	return nil
}
