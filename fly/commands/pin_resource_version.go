package commands

import (
	"fmt"
	"github.com/concourse/concourse/fly/commands/internal/displayhelpers"
	"github.com/concourse/concourse/fly/commands/internal/flaghelpers"
	"github.com/concourse/concourse/fly/rc"
)

type PinResourceVersionCommand struct {
	Resource flaghelpers.ResourceFlag `short:"r" long:"resource" required:"true" value-name:"PIPELINE/RESOURCE" description:"Name of the resource"`
	ResourceVersionID int `short:"i" long:"version-id" required:"true" description:"ID of the version"`
}


func (command *PinResourceVersionCommand) Execute([]string) error {
	target, err := rc.LoadTarget(Fly.Target, Fly.Verbose)
	if err != nil {
		return err
	}

	err = target.Validate()
	if err != nil {
		return err
	}

	team := target.Team()

	pinned, err := team.PinResourceVersion(command.Resource.PipelineName, command.Resource.ResourceName, command.ResourceVersionID)

	if err != nil {
		return err
	}

	if pinned {
		fmt.Printf("pinned '%s/%s' at version id %d\n", command.Resource.PipelineName, command.Resource.ResourceName, command.ResourceVersionID)
	} else {
		displayhelpers.Failf("could not pin '%s/%s' at version %d, make sure the resource and version exists\n",
			command.Resource.PipelineName, command.Resource.ResourceName, command.ResourceVersionID)
	}

	return nil
}
